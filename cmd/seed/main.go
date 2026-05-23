// Command seed populates a wachat DB with a handful of chats and varied
// messages so the UI has something to render. Intended for local demo /
// manual testing; not part of the production app.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/jdtoon/wachat/internal/store"
)

type chatSeed struct {
	jid, name string
	sender    string // "me" or that person's jid
}

var demoChats = []chatSeed{
	{jid: "alice@s.whatsapp.net", name: "Alice", sender: "alice@s.whatsapp.net"},
	{jid: "bob@s.whatsapp.net", name: "Bob Carter", sender: "bob@s.whatsapp.net"},
	{jid: "family-group@g.us", name: "Family ❤", sender: "mom@s.whatsapp.net"},
	{jid: "work-team@g.us", name: "Work — engineering", sender: "alex@s.whatsapp.net"},
	{jid: "ada@s.whatsapp.net", name: "Ada Lovelace", sender: "ada@s.whatsapp.net"},
}

var sampleMessages = []string{
	"hey, are you free tonight?",
	"sure — what time?",
	"I'll send the link in a sec",
	"thx 🙏",
	"that meeting was painful",
	"haha agreed",
	"running 10m late, sorry",
	"no worries, see you soon",
	"check this out: https://example.com",
	"who's bringing dessert?",
	"I can pick something up",
	"yes please",
	"weather is awful today",
	"stay warm out there",
	"new bench shows 0.5MB heap for 100k msgs 👀",
	"that's basically free",
	"shipping the v0 today",
	"🥳",
}

func main() {
	dbPath := flag.String("db", "wachat.db", "path to the wachat DB to seed")
	perChat := flag.Int("n", 40, "messages per chat")
	flag.Parse()

	if err := run(*dbPath, *perChat); err != nil {
		log.Fatal(err)
	}
}

func run(dbPath string, perChat int) error {
	ctx := context.Background()
	s, err := store.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = s.Close() }()

	rng := rand.New(rand.NewSource(42))
	now := time.Now().UnixMilli()

	// Insert messages spaced across the last ~3 days so the humanTime
	// formatter shows a mix of "Xm ago", "Xh ago", and dates.
	for ci, c := range demoChats {
		if err := s.UpsertChat(ctx, c.jid, c.name); err != nil {
			return err
		}
		// Each chat's newest message lands at a different offset so the
		// chat list has a clear ordering.
		newestOffset := time.Duration(ci) * 30 * time.Minute
		for i := 0; i < perChat; i++ {
			// Older messages spread further back in time.
			ageMin := time.Duration(i)*7*time.Minute + newestOffset
			ts := now - ageMin.Milliseconds()

			from := c.sender
			if i%3 == 0 {
				from = "" // "from me" — empty sender JID
			}
			body := sampleMessages[rng.Intn(len(sampleMessages))]

			waID := fmt.Sprintf("seed-%s-%05d", c.jid, i)
			if _, err := s.Insert(ctx, store.Message{
				WAID:      waID,
				ChatJID:   c.jid,
				SenderJID: from,
				TS:        ts,
				Body:      body,
			}, from != ""); err != nil {
				return fmt.Errorf("insert: %w", err)
			}
		}
	}

	fmt.Printf("seeded %d chats x %d messages into %s\n", len(demoChats), perChat, dbPath)
	return nil
}
