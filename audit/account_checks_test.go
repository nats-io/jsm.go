package audit

import (
	"path/filepath"
	"testing"

	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/jwt/v2"
	"github.com/nats-io/nats-server/v2/server"
)

func setupAccountCheck(t *testing.T, checkid string, accountz *server.ServerAPIAccountzResponse, details map[string]*server.AccountInfo) Outcome {
	tmp := t.TempDir()
	archivePath := filepath.Join(tmp, "audit.zip")

	writer, err := archive.NewWriter(archivePath)
	if err != nil {
		t.Fatalf("failed to create archive writer: %v", err)
	}

	if err := writer.Add(accountz, archive.TagCluster("C1"), archive.TagServer("S1"), archive.TagServerAccounts()); err != nil {
		t.Fatalf("failed to add accountz: %v", err)
	}

	for account, info := range details {
		if err := writer.Add(info, archive.TagCluster("C1"), archive.TagServer("S1"), archive.TagAccount(account), archive.TagAccountInfo()); err != nil {
			t.Fatalf("failed to add account info: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("failed to close archive writer: %v", err)
	}

	reader, err := archive.NewReader(archivePath)
	if err != nil {
		t.Fatalf("failed to open archive: %v", err)
	}
	defer reader.Close()

	cc := &CheckCollection{}
	if err := RegisterAccountChecks(cc); err != nil {
		t.Fatalf("failed to register checks: %v", err)
	}

	var check *Check
	cc.EachCheck(func(c *Check) {
		if c.Code == checkid {
			check = c
		}
	})
	if check == nil {
		t.Fatalf("check %s not found", checkid)
	}

	examples := newExamplesCollection(0)
	result, err := check.Handler(check, reader, examples, api.NewDefaultLogger(api.ErrorLevel))
	if err != nil {
		t.Fatalf("check handler failed: %v", err)
	}

	return result
}

func TestACCOUNTS_001(t *testing.T) {
	t.Run("Should warn if account is close to connection limit", func(t *testing.T) {
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					ClientCnt:   901,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								AccountLimits: jwt.AccountLimits{Conn: 1000},
							},
						},
					},
				},
			},
		)
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should warn if account is close to subscription limit", func(t *testing.T) {
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					SubCnt:      901,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								NatsLimits: jwt.NatsLimits{Subs: 1000},
							},
						},
					},
				},
			},
		)
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should warn if account is close to leaf connection limit", func(t *testing.T) {
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					LeafCnt:     901,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								AccountLimits: jwt.AccountLimits{LeafNodeConn: 1000},
							},
						},
					},
				},
			},
		)
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should pass if account is within limits", func(t *testing.T) {
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					ClientCnt:   100,
					SubCnt:      100,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								NatsLimits:    jwt.NatsLimits{Subs: 1000},
								AccountLimits: jwt.AccountLimits{Conn: 1000},
							},
						},
					},
				},
			},
		)
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should pass if account has no JWT claim", func(t *testing.T) {
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					ClientCnt:   999,
					Claim:       nil,
				},
			},
		)
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should pass if limit is zero (unlimited)", func(t *testing.T) {
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					ClientCnt:   9999,
					SubCnt:      9999,
					LeafCnt:     9999,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								NatsLimits:    jwt.NatsLimits{Subs: 0},
								AccountLimits: jwt.AccountLimits{Conn: 0, LeafNodeConn: 0},
							},
						},
					},
				},
			},
		)
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should pass at exactly the threshold boundary", func(t *testing.T) {
		// With 90% threshold and limit=1000, threshold=900; value=900 must not trigger
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					ClientCnt:   900,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								AccountLimits: jwt.AccountLimits{Conn: 1000},
							},
						},
					},
				},
			},
		)
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})

	t.Run("Should warn one above the threshold boundary", func(t *testing.T) {
		// With 90% threshold and limit=1000, threshold=900; value=901 must trigger
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					ClientCnt:   901,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								AccountLimits: jwt.AccountLimits{Conn: 1000},
							},
						},
					},
				},
			},
		)
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should fail if usage exceeds limit", func(t *testing.T) {
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					AccountName: "SYS",
					ClientCnt:   1001,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								AccountLimits: jwt.AccountLimits{Conn: 1000},
							},
						},
					},
				},
			},
		)
		if result != Fail {
			t.Errorf("expected result %v, got %v", Fail, result)
		}
	})

	t.Run("Should reflect the worst outcome across multiple accounts", func(t *testing.T) {
		// APP is within limits, SYS is approaching — expect PassWithIssues
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"APP", "SYS"}},
			},
			map[string]*server.AccountInfo{
				"APP": {
					AccountName: "APP",
					ClientCnt:   100,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								AccountLimits: jwt.AccountLimits{Conn: 1000},
							},
						},
					},
				},
				"SYS": {
					AccountName: "SYS",
					ClientCnt:   901,
					Claim: &jwt.AccountClaims{
						Account: jwt.Account{
							Limits: jwt.OperatorLimits{
								AccountLimits: jwt.AccountLimits{Conn: 1000},
							},
						},
					},
				},
			},
		)
		if result != PassWithIssues {
			t.Errorf("expected result %v, got %v", PassWithIssues, result)
		}
	})

	t.Run("Should handle missing account info gracefully", func(t *testing.T) {
		tmp := t.TempDir()
		archivePath := filepath.Join(tmp, "audit.zip")

		writer, err := archive.NewWriter(archivePath)
		if err != nil {
			t.Fatalf("failed to create archive writer: %v", err)
		}

		// SYS has full account info and is within limits
		sysInfo := &server.AccountInfo{
			AccountName: "SYS",
			ClientCnt:   100,
			Claim: &jwt.AccountClaims{
				Account: jwt.Account{
					Limits: jwt.OperatorLimits{
						AccountLimits: jwt.AccountLimits{Conn: 1000},
					},
				},
			},
		}
		if err := writer.Add(sysInfo, archive.TagCluster("C1"), archive.TagServer("S1"), archive.TagAccount("SYS"), archive.TagAccountInfo()); err != nil {
			t.Fatalf("failed to add SYS account info: %v", err)
		}

		// ORPHAN is indexed via a TagAccount entry paired with a non-AccountInfo type tag,
		// so ForEachTaggedArtifact filtering on TagAccountInfo() returns ErrNoMatches.
		// The check must skip it gracefully rather than aborting.
		orphanAccountz := &server.ServerAPIAccountzResponse{
			Data: &server.Accountz{Accounts: []string{"ORPHAN"}},
		}
		if err := writer.Add(orphanAccountz, archive.TagCluster("C1"), archive.TagServer("S1"), archive.TagAccount("ORPHAN"), archive.TagServerAccounts()); err != nil {
			t.Fatalf("failed to add ORPHAN account entry: %v", err)
		}

		if err := writer.Close(); err != nil {
			t.Fatalf("failed to close archive writer: %v", err)
		}

		reader, err := archive.NewReader(archivePath)
		if err != nil {
			t.Fatalf("failed to open archive: %v", err)
		}
		defer reader.Close()

		cc := &CheckCollection{}
		if err := RegisterAccountChecks(cc); err != nil {
			t.Fatalf("failed to register checks: %v", err)
		}

		var check *Check
		cc.EachCheck(func(c *Check) {
			if c.Code == "ACCOUNTS_001" {
				check = c
			}
		})
		if check == nil {
			t.Fatalf("check ACCOUNTS_001 not found")
		}

		examples := newExamplesCollection(0)
		result, err := check.Handler(check, reader, examples, api.NewDefaultLogger(api.ErrorLevel))
		if err != nil {
			t.Fatalf("check handler failed unexpectedly: %v", err)
		}

		// SYS is within limits, ORPHAN missing info is skipped — overall Pass
		if result != Pass {
			t.Errorf("expected result %v, got %v", Pass, result)
		}
	})
}
