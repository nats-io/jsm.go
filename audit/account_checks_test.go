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
					ClientCnt: 901,
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
					SubCnt: 901,
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

	t.Run("Should pass if account is within limits", func(t *testing.T) {
		result := setupAccountCheck(t, "ACCOUNTS_001",
			&server.ServerAPIAccountzResponse{
				Data: &server.Accountz{Accounts: []string{"SYS"}},
			},
			map[string]*server.AccountInfo{
				"SYS": {
					ClientCnt: 100,
					SubCnt:    100,
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
}
