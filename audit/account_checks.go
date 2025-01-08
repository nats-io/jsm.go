// Copyright 2024-2025 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package audit

import (
	"errors"
	"fmt"
	"github.com/nats-io/jsm.go/api"
	"github.com/nats-io/jsm.go/audit/archive"
	"github.com/nats-io/nats-server/v2/server"
)

func RegisterAccountChecks(collection *CheckCollection) error {
	return collection.Register(Check{
		Code:        "ACCOUNTS_001",
		Suite:       "accounts",
		Name:        "Account Limits",
		Description: "Account usage is below the configured limits",
		Configuration: map[string]*CheckConfiguration{
			"connections": {
				Key:         "connections",
				Description: "Alerting threshold as a fraction of configured connections limit",
				Unit:        PercentageUnit,
				Default:     0.9,
			},
			"subscriptions": {
				Key:         "subscriptions",
				Description: "Alerting threshold as a fraction of configured subscriptions limit",
				Unit:        PercentageUnit,
				Default:     0.9,
			},
		},
		Handler: checkAccountLimits,
	})
}

// checkAccountLimits verifies that the number of connections & subscriptions is not approaching the limit set for the account
func checkAccountLimits(check *Check, r *archive.Reader, examples *ExamplesCollection, log api.Logger) (Outcome, error) {
	connectionsThreshold := check.Configuration["connections"].Value()
	subscriptionsThreshold := check.Configuration["subscriptions"].Value()
	accountDetailsTag := archive.TagAccountInfo()

	// Check value against limit threshold, create example if exceeded
	checkLimit := func(limitName, serverName, accountName string, value, limit int64, percentThreshold float64) {
		if limit <= 0 {
			// Limit not set
			return
		}

		threshold := int64(float64(limit) * percentThreshold)
		if value > threshold {
			examples.Add("account %s (on %s) using %.1f%% of %s limit (%d/%d)", accountName, serverName, float64(value)*100/float64(limit), limitName, value, limit)
		}
	}

	_, err := r.EachClusterServerAccountz(func(clusterTag *archive.Tag, serverTag *archive.Tag, err error, az *server.ServerAPIAccountzResponse) error {
		if errors.Is(err, archive.ErrNoMatches) {
			log.Warnf("Artifact 'ACCOUNTZ' is missing for server %s", serverTag)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to load variables for server %s: %w", serverTag, err)
		}

		accountz := az.Data
		for _, accountName := range accountz.Accounts {
			accountTag := archive.TagAccount(accountName)

			var accountInfo server.AccountInfo
			err := r.Load(&accountInfo, serverTag, accountTag, accountDetailsTag)
			if errors.Is(err, archive.ErrNoMatches) {
				log.Warnf("Account details is missing for account %s, server %s", accountName, serverTag)
				continue
			} else if err != nil {
				return fmt.Errorf("failed to load Account details from server %s for account %s, error: %w", serverTag, accountName, err)
			}

			if accountInfo.Claim == nil {
				// Can't check limits without a claim
				continue
			}

			checkLimit(
				"client connections",
				serverTag.Value,
				accountName,
				int64(accountInfo.ClientCnt),
				accountInfo.Claim.Limits.Conn,
				connectionsThreshold,
			)

			checkLimit(
				"client connections (account)",
				serverTag.Value,
				accountName,
				int64(accountInfo.ClientCnt),
				accountInfo.Claim.Limits.AccountLimits.Conn,
				connectionsThreshold,
			)

			checkLimit(
				"leaf connections",
				serverTag.Value,
				accountName,
				int64(accountInfo.LeafCnt),
				accountInfo.Claim.Limits.LeafNodeConn,
				connectionsThreshold,
			)

			checkLimit(
				"leaf connections (account)",
				serverTag.Value,
				accountName,
				int64(accountInfo.LeafCnt),
				accountInfo.Claim.Limits.AccountLimits.LeafNodeConn,
				connectionsThreshold,
			)

			checkLimit(
				"subscriptions",
				serverTag.Value,
				accountName,
				int64(accountInfo.SubCnt),
				accountInfo.Claim.Limits.Subs,
				subscriptionsThreshold,
			)
		}

		return nil
	})
	if err != nil {
		return Skipped, err
	}

	if examples.Count() > 0 {
		log.Errorf("Found %d instances of accounts approaching limit", examples.Count())
		return PassWithIssues, nil
	}

	return Pass, nil
}
