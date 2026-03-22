package main

import (
	"context"
	"log"
	"regexp"
	"strings"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var badWords = []string{
	"хуй", "пизда", "бля", "ебать", "ёбаный", "нахуй", "пиздец",
	"пидор", "пидорас", "мудак", "говно", "ПИДОРАС", "сука",
}
var matcher = regexp.MustCompile(`(?i)(` + strings.Join(badWords, "|") + `)`)

func main() {
	// ===== НАСТРОЙКИ =====
	homeserver := "http://localhost:8008"
	userID := "@sroot:linux.gnu"
	password := "root09root"
	// =====================

	cli, err := mautrix.NewClient(homeserver, id.UserID(userID), "")
	if err != nil {
		log.Fatalf("Не удалось создать клиент: %v", err)
	}

	resp, err := cli.Login(context.Background(), &mautrix.ReqLogin{
		Type:       "m.login.password",
		Identifier: mautrix.UserIdentifier{Type: "m.id.user", User: userID},
		Password:   password,
	})
	if err != nil {
		log.Fatalf("Ошибка логина: %v", err)
	}
	log.Printf("Залогинились как %s", resp.UserID)
	cli.AccessToken = resp.AccessToken

	// Получаем список комнат (не критично, можно опустить)
	rooms, err := cli.JoinedRooms(context.Background())
	if err != nil {
		log.Printf("Не удалось получить комнаты: %v", err)
	} else {
		log.Printf("Бот состоит в комнатах: %v", rooms.JoinedRooms)
	}

	syncer := cli.Syncer.(*mautrix.DefaultSyncer)

	// Обработка приглашений – тоже в горутине, чтобы не блокировать синхронизацию
	syncer.OnEventType(event.StateMember, func(ctx context.Context, evt *event.Event) {
		go func() {
			// Небольшая задержка для стабильности, можно убрать
			time.Sleep(100 * time.Millisecond)

			memberContent, ok := evt.Content.Parsed.(*event.MemberEventContent)
			if !ok {
				return
			}
			if evt.GetStateKey() == cli.UserID.String() && memberContent.Membership == "invite" {
				log.Printf("Получено приглашение в комнату %s от %s", evt.RoomID, evt.Sender)
				// Создаём отдельный контекст с таймаутом, чтобы не зависнуть
				joinCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				_, err := cli.JoinRoom(joinCtx, evt.RoomID.String(), nil)
				if err != nil {
					log.Printf("Ошибка при присоединении к комнате %s: %v", evt.RoomID, err)
				} else {
					log.Printf("Успешно присоединились к комнате %s", evt.RoomID)
				}
			}
		}()
	})

	// Обработка сообщений с матом – тоже в горутине
	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		go func() {
			if evt.Sender == cli.UserID {
				return
			}
			msg := evt.Content.AsMessage()
			if msg == nil {
				return
			}

			if matcher.MatchString(msg.Body) {
				log.Printf("Мат от %s в комнате %s: %s", evt.Sender, evt.RoomID, msg.Body)

				// Контекст для операций – 5 секунд на каждую
				sendCtx, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel1()

				// Отправляем предупреждение
				_, err := cli.SendMessageEvent(sendCtx, evt.RoomID, event.EventMessage, &event.MessageEventContent{
					MsgType:       event.MsgText,
					Body:          "ПИДОРАС НЕ МАТЕРИСЬ",
					Format:        event.FormatHTML,
					FormattedBody: "<b>ПИДОРАС НЕ МАТЕРИСЬ</b>",
				})
				if err != nil {
					log.Printf("Ошибка отправки ответа: %v", err)
				}

				redactCtx, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel2()

				// Удаляем сообщение с матом
				_, err = cli.RedactEvent(redactCtx, evt.RoomID, evt.ID, mautrix.ReqRedact{Reason: "мат"})
				if err != nil {
					log.Printf("Ошибка удаления сообщения: %v", err)
				} else {
					log.Printf("Сообщение удалено")
				}
			}
		}()
	})

	log.Println("Бот запущен, обрабатывает сообщения и приглашения максимально быстро...")
	if err = cli.SyncWithContext(context.Background()); err != nil {
		log.Fatalf("Ошибка синхронизации: %v", err)
	}
}
