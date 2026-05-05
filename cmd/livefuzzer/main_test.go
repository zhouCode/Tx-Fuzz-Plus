package main

import (
	"testing"

	"github.com/MariusVanDerWijden/tx-fuzz/flags"
)

func TestInitAppRegistersCampaignAndReplayCommands(t *testing.T) {
	app := initApp()
	seenCampaign := false
	seenReplay := false
	for _, cmd := range app.Commands {

		if cmd.Name == "airdrop" {
			foundCount := false
			for _, flag := range cmd.Flags {
				if flag.Names()[0] == flags.CountFlag.Name {
					foundCount = true
				}
			}
			if !foundCount {
				t.Fatalf("airdrop command missing %s flag", flags.CountFlag.Name)
			}
		}
		if cmd.Name == "campaign" {
			seenCampaign = true
			if len(cmd.Subcommands) != 3 {
				t.Fatalf("unexpected campaign subcommand count: %d", len(cmd.Subcommands))
			}
			for _, sub := range cmd.Subcommands {
				if got := len(sub.Flags); got != len(flags.CampaignFlags) {
					t.Fatalf("campaign %s flags mismatch: got=%d want=%d", sub.Name, got, len(flags.CampaignFlags))
				}
			}
		}
		if cmd.Name == "replay" {
			seenReplay = true
			foundBundle := false
			foundEndpoints := false
			for _, flag := range cmd.Flags {
				switch flag.Names()[0] {
				case flags.BundleFlag.Name:
					foundBundle = true
				case flags.EndpointsFlag.Name:
					foundEndpoints = true
				}
			}
			if !foundBundle || !foundEndpoints {
				t.Fatalf("replay flags missing bundle/endpoints: bundle=%v endpoints=%v", foundBundle, foundEndpoints)
			}
		}
	}
	if !seenCampaign || !seenReplay {
		t.Fatalf("expected campaign and replay commands, got campaign=%v replay=%v", seenCampaign, seenReplay)
	}
}

func TestCampaignCommandRegistersAllFamilies(t *testing.T) {
	app := initApp()
	for _, cmd := range app.Commands {
		if cmd.Name != "campaign" {
			continue
		}
		seen := map[string]bool{}
		for _, sub := range cmd.Subcommands {
			seen[sub.Name] = true
		}
		for _, want := range []string{"basic", "blob", "pectra"} {
			if !seen[want] {
				t.Fatalf("campaign missing %s subcommand: %#v", want, seen)
			}
		}
		return
	}
	t.Fatal("campaign command not found")
}

func TestCampaignCommandRegistersD1ExecutionFlags(t *testing.T) {
	app := initApp()
	for _, cmd := range app.Commands {
		if cmd.Name != "campaign" {
			continue
		}
		for _, sub := range cmd.Subcommands {
			if sub.Name != "basic" {
				continue
			}
			seen := map[string]bool{}
			for _, flag := range sub.Flags {
				seen[flag.Names()[0]] = true
			}
			for _, want := range []string{
				flags.ExecutionModeFlag.Name,
				flags.MaxInFlightFlag.Name,
				flags.ConfirmSLAFlag.Name,
				flags.ConfirmDrainTimeoutFlag.Name,
			} {
				if !seen[want] {
					t.Fatalf("basic campaign missing %s flag: %#v", want, seen)
				}
			}
			return
		}
	}
	t.Fatal("campaign basic command not found")
}
