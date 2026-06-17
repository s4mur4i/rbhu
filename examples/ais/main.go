// Command ais demonstrates creating an AIS consent with the rbhu helpers.
//
//	go run ./examples/ais
//
// It loads credentials from secrets/.env and the client certificate from
// secrets/certificate_RBHU_SB_KONG_PROD.p12, creates a consent for a sandbox
// test account, and prints the URL the PSU must visit to authorize it (SCA).
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/s4mur4i/rbhu"
)

func main() {
	client, err := rbhu.NewSandboxFromEnv("", "") // defaults to secrets/
	if err != nil {
		log.Fatalf("setup: %v", err)
	}

	iban := os.Getenv("RBHU_TEST_IBAN")
	if iban == "" {
		iban = "HU19120010080010059400100008" // documented sandbox test account
	}

	consent, err := client.CreateAISConsent(context.Background(), rbhu.AISConsentParams{
		IBANs:     []string{iban},
		PSUID:     "82742150",
		Recurring: true,
	})
	if err != nil {
		log.Fatalf("create consent: %v", err)
	}

	fmt.Println("consentId:", consent.ID)
	fmt.Println("status:   ", consent.Status)
	fmt.Println("\nHave the PSU open this URL to authorize (SCA):")
	fmt.Println(client.AuthorizeURL(rbhu.ScopeAISP, consent.ID, "example-state"))
	fmt.Println("\nAfter SCA, exchange the ?code= for a token (see examples/sca).")
}
