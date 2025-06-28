package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/fiatjaf/eventstore/postgresql"
	"github.com/joho/godotenv"
	"github.com/nbd-wtf/go-nostr"
	glog "gopkg.in/op/go-logging.v1"
)

func createPaginator() func(ctx context.Context, config *AppConfig) chan nostr.RelayEvent {
	return func(ctx context.Context, config *AppConfig) chan nostr.RelayEvent {
		ch := make(chan nostr.RelayEvent, config.Filter.Limit)

		var wg sync.WaitGroup

		for _, rd := range config.Relays {
			interval := time.Duration(rd.Interval) * time.Second
			since, err := time.Parse("January 2, 2006 15:04:05", rd.Until)
			if err != nil {
				break
			}
			wg.Add(1)

			go func() {
				defer wg.Done()

				relay, err := nostr.RelayConnect(ctx, rd.URL)
				if err != nil {
					return
				}

				nextUntil := nostr.Timestamp(since.Unix())
				filter := config.Filter

				for {
					filter.Until = &nextUntil

					config.UpdateUntil(rd.URL, filter.Until.Time())

					sub, err := relay.Subscribe(ctx, nostr.Filters{filter})
					if err != nil {
						return
					}

					keepGoing := false
				loop:
					for {
						select {
						case event, more := <-sub.Events:
							if !more {
								break loop
							}
							keepGoing = true
							relayEvent := nostr.RelayEvent{Event: event, Relay: relay}
							ch <- relayEvent

							if event.CreatedAt < *filter.Until {
								nextUntil = event.CreatedAt
							}
						case <-sub.ClosedReason:
							sub.Unsub()
							break loop
						case <-sub.EndOfStoredEvents:
							sub.Unsub()
							break loop
						case <-ctx.Done():
							sub.Unsub()
							return
						}
					}
					if !keepGoing {
						return
					}
					time.Sleep(interval)
				}
			}()
		}

		go func() {
			wg.Wait()
			close(ch)
		}()

		return ch
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	path := "config.yaml"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	config, err := Load(path)

	if err != nil {
		log.Fatalf("Error loading config.yaml %v\n", err)
	}

	fmt.Println("Harvesting...")

	store := postgresql.PostgresBackend{DatabaseURL: os.Getenv("DATABASE_URL")}
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	nostr.InfoLogger = log.New(io.Discard, "", 0)
	glog.SetLevel(glog.WARNING, "yq-lib")

	_, uiCh := InitializeTUI(config.Filter.Limit)

	err = store.Init()
	if err != nil {
		panic(err)
	}

	paginator := createPaginator()

	for ie := range paginator(ctx, config) {
		if _, err := ie.Event.CheckSignature(); err != nil {
			continue
		}
		if !config.Filter.Matches(ie.Event) {
			continue
		}
		uiCh <- UIRelayEventMsg{
			url:        ie.Relay.URL,
			kind:       ie.Kind,
			created_at: int64(ie.CreatedAt),
		}
		err := store.SaveEvent(ctx, ie.Event)
		if err != nil {
			continue
		} else {
			uiCh <- UIInsertMsg{}
		}
	}
}
