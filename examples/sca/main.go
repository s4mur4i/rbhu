// Command sca runs the full AIS flow end to end with the rbhu helpers:
// create consent, complete SCA (capturing the redirect on a local server),
// exchange the code for a token, and read the account list, balances and
// transactions.
//
//	go run ./examples/sca
//
// The application's redirect URI in the API Marketplace must point at the local
// callback below (default http://127.0.0.1:8089/callback). Override with
// RBHU_CALLBACK_ADDR (host:port) and ensure the registered redirect_url in
// secrets/.env matches.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/s4mur4i/rbhu"
)

func main() {
	client, err := rbhu.NewSandboxFromEnv("", "")
	if err != nil {
		log.Fatalf("setup: %v", err)
	}

	iban := envOr("RBHU_TEST_IBAN", "HU19120010080010059400100008")
	psuID := envOr("RBHU_TEST_PSU_ID", "82742150")
	addr := envOr("RBHU_CALLBACK_ADDR", "127.0.0.1:8089")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	consent, err := client.CreateAISConsent(ctx, rbhu.AISConsentParams{
		IBANs: []string{iban}, PSUID: psuID, Recurring: true,
	})
	if err != nil {
		log.Fatalf("create consent: %v", err)
	}
	fmt.Println("consentId:", consent.ID)

	// Open the authorize URL (browser) and capture the code on the local server.
	_, err = client.CompleteAuthorization(ctx, rbhu.ScopeAISP, consent.ID, addr, func(authURL string) {
		fmt.Println("\nAuthorize here (opening browser):")
		fmt.Println(authURL)
		openBrowser(authURL)
	})
	if err != nil {
		log.Fatalf("authorization: %v", err)
	}
	fmt.Println("token obtained.")

	accts, err := client.ListAccounts(ctx, consent.ID)
	if err != nil {
		log.Fatalf("list accounts: %v", err)
	}
	for _, a := range accts {
		fmt.Printf("\naccount %s (%s)\n", a.ResourceId, a.Currency)
		if bal, err := client.Balances(ctx, consent.ID, a.ResourceId); err == nil && bal.Balances != nil {
			fmt.Printf("  balances: %d entries\n", len(*bal.Balances))
		}
		if tx, err := client.Transactions(ctx, consent.ID, a.ResourceId, "booked"); err == nil {
			_ = tx
			fmt.Println("  transactions fetched")
		}
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func openBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	_ = exec.Command(cmd, append(args, url)...).Start()
}
