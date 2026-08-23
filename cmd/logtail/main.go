// logtail ek dev tool hai: kisi deployment ke live build logs terminal me
// dikhata hai, bilkul waise jaise dashboard dikhayega. Frontend ke bina
// centrifugo ka poora raasta test karne ke liye.
//
//	go run ./cmd/logtail --config ./config/local.yaml --deployment deployment_xxx
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/centrifugal/centrifuge-go"
	"github.com/mistryviral05/dockyard/internal/config"
	postgres "github.com/mistryviral05/dockyard/internal/http/database"
	"github.com/mistryviral05/dockyard/internal/http/utils/centrifugo"
	"github.com/mistryviral05/dockyard/internal/http/utils/signer"
)

// logMessage worker ke publish kiye message ka shape hai.
type logMessage struct {
	Seq   int      `json:"seq"`
	Type  string   `json:"type"`
	Lines []string `json:"lines"`
	At    string   `json:"at"`
}

const (
	reset  = "\033[0m"
	dim    = "\033[2m"
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
)

func main() {
	// ye flags MustLoad se pehle register hone chahiye: wahi flag.Parse() karta hai
	deploymentID := flag.String("deployment", "", "deployment id to tail")
	clerkID := flag.String("user", "", "clerk id to connect as (default: the deployment's owner)")
	showHistory := flag.Bool("history", true, "print what is already in the channel history first")

	cfg := config.MustLoad()

	// CONFIG_PATH set ho to MustLoad flag.Parse() nahi karta
	if !flag.Parsed() {
		flag.Parse()
	}

	if *deploymentID == "" {
		fmt.Fprintln(os.Stderr, "usage: logtail --config ./config/local.yaml --deployment <deployment_id>")
		os.Exit(2)
	}

	db, err := postgres.New(cfg.Database)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	owner := *clerkID
	if owner == "" {
		owner, err = ownerOf(db, *deploymentID)
		if err != nil {
			log.Fatalf("could not find the owner of %s: %v", *deploymentID, err)
		}
	}

	// wahi token jo /api/v1/centrifugo/token deta hai.
	// go-jose ko struct chahiye, map nahi.
	claims := struct {
		Sub string `json:"sub"`
	}{Sub: owner}

	token, err := signer.New(cfg.Centrifugo.TokenSecret).
		Sign(claims, time.Now().Add(cfg.Centrifugo.TokenTTL))
	if err != nil {
		log.Fatalf("sign token: %v", err)
	}

	channel := centrifugo.DeploymentChannel(*deploymentID)

	fmt.Printf("%s  channel   %s%s\n", dim, channel, reset)
	fmt.Printf("%s  user      %s%s\n", dim, owner, reset)
	fmt.Printf("%s  server    %s%s\n\n", dim, cfg.Centrifugo.WSUrl, reset)

	client := centrifuge.NewJsonClient(cfg.Centrifugo.WSUrl, centrifuge.Config{Token: token})
	defer client.Close()

	client.OnConnected(func(e centrifuge.ConnectedEvent) {
		fmt.Printf("%s  connected as %s%s\n", green, e.ClientID, reset)
	})
	client.OnDisconnected(func(e centrifuge.DisconnectedEvent) {
		fmt.Printf("%s  disconnected: %s%s\n", yellow, e.Reason, reset)
	})
	client.OnError(func(e centrifuge.ErrorEvent) {
		fmt.Printf("%s  client error: %s%s\n", red, e.Error, reset)
	})

	sub, err := client.NewSubscription(channel, centrifuge.SubscriptionConfig{
		Recoverable: true,
		Positioned:  true,
	})
	if err != nil {
		log.Fatalf("subscription: %v", err)
	}

	sub.OnSubscribed(func(centrifuge.SubscribedEvent) {
		fmt.Printf("%s  subscribed%s\n\n", green, reset)

		if !*showHistory {
			return
		}

		// History alag goroutine me: ye callback client ke event loop pe
		// chalta hai, aur wahi loop history ka reply bhi padhta hai. Yahin
		// se call karoge to wo apne hi jawab ka intezaar karta reh jayega.
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			res, err := sub.History(ctx, centrifuge.WithHistoryLimit(500))
			if err != nil {
				fmt.Printf("%s  history unavailable: %s%s\n", yellow, err, reset)
				return
			}
			for _, p := range res.Publications {
				print(p.Data)
			}
			fmt.Printf("%s  ---- %d from history, now live ----%s\n", dim, len(res.Publications), reset)
		}()
	})

	// yahi wo error hai jo subscribe proxy ke deny karne pe aata hai
	sub.OnError(func(e centrifuge.SubscriptionErrorEvent) {
		fmt.Printf("%s  subscribe failed: %s%s\n", red, e.Error, reset)
	})

	sub.OnPublication(func(e centrifuge.PublicationEvent) {
		print(e.Data)
	})

	if err := sub.Subscribe(); err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	if err := client.Connect(); err != nil {
		log.Fatalf("connect: %v", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-done

	fmt.Printf("\n%s  bye%s\n", dim, reset)
}

func print(data []byte) {
	var msg logMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		fmt.Printf("%s  raw: %s%s\n", yellow, data, reset)
		return
	}

	tag := fmt.Sprintf("%s%-6s%s", cyan, msg.Type, reset)
	if msg.Type == "system" {
		tag = fmt.Sprintf("%s%-6s%s", green, msg.Type, reset)
	}

	for _, line := range msg.Lines {
		fmt.Printf("  %s %s%4d%s  %s\n", tag, dim, msg.Seq, reset, line)
	}
}

// ownerOf deployment ka clerk id nikalta hai. Wahi ownership join jo
// repository use karti hai, taki tool asli permission path hi test kare.
func ownerOf(db *sql.DB, deploymentID string) (string, error) {
	var clerkID string

	err := db.QueryRow(`
		SELECT u.clerk_id
		FROM deployments d
		JOIN projects p   ON p.id = d.project_id
		JOIN workspaces w ON w.id = p.wsp_id
		JOIN users u      ON u.id = w.user_id
		WHERE d.id = $1
	`, deploymentID).Scan(&clerkID)

	return clerkID, err
}
