// SPDX-License-Identifier: ice License 1.0

package miner

import (
	"context"
	_ "embed"
	"io"
	"sync"
	"sync/atomic"
	stdlibtime "time"

	dwh "github.com/ice-blockchain/freezer/bookkeeper/storage"
	"github.com/ice-blockchain/freezer/model"
	"github.com/ice-blockchain/freezer/tokenomics"
	messagebroker "github.com/ice-blockchain/wintr/connectors/message_broker"
	storagePG "github.com/ice-blockchain/wintr/connectors/storage/v2"
	"github.com/ice-blockchain/wintr/connectors/storage/v3"
	"github.com/ice-blockchain/wintr/time"
)

// Public API.

type (
	Client interface {
		io.Closer
		CheckHealth(context.Context) error
	}
	DayOffStarted struct {
		StartedAt                   *time.Time `json:"startedAt,omitempty"`
		EndedAt                     *time.Time `json:"endedAt,omitempty"`
		UserID                      string     `json:"userId,omitempty" `
		ID                          string     `json:"id,omitempty"`
		RemainingFreeMiningSessions uint64     `json:"remainingFreeMiningSessions,omitempty"`
		MiningStreak                uint64     `json:"miningStreak,omitempty"`
	}
)

// Private API.

const (
	applicationYamlKey       = "miner"
	parentApplicationYamlKey = "tokenomics"
	requestDeadline          = 30 * stdlibtime.Second

	startRecalculationsFrom = "2023-11-20T14:00:00"
	timeLayout              = "2006-01-02T15:04:05"

	// Dry run.
	balanceForTMinusBugfixDryRunEnabled = false
	balanceT2BugfixDryRunEnabled        = false

	// !!! [CRUCUAL] Real run if this is TRUE and DRY RUN false.
	balanceForTMinusBugfixEnabled = false
	balanceT2BugfixEnabled        = false

	clearBugfixDebugInfoEnabled = true
)

// .
var (
	//nolint:gochecknoglobals // Singleton & global config mounted only during bootstrap.
	cfg config

	//go:embed .testdata/DDL.sql
	eskimoDDL string
)

type (
	user struct {
		model.MiningSessionSoloLastStartedAtField
		model.MiningSessionSoloStartedAtField
		model.MiningSessionSoloEndedAtField
		model.MiningSessionSoloPreviouslyEndedAtField
		model.ExtraBonusStartedAtField
		model.LatestDeviceField
		model.UserIDField
		UpdatedUser
		model.BalanceSoloPendingField
		model.BalanceT1PendingField
		model.BalanceT2PendingField
		model.ActiveT1ReferralsField
		model.ActiveT2ReferralsField
		model.PreStakingBonusField
		model.PreStakingAllocationField
		model.ExtraBonusField
		model.UTCOffsetField
		model.ReferralsCountChangeGuardUpdatedAtField
	}

	UpdatedUser struct { // This is public only because we have to embed it, and it has to be if so.
		model.ExtraBonusLastClaimAvailableAtField
		model.BalanceLastUpdatedAtField
		model.ResurrectSoloUsedAtField
		model.ResurrectT0UsedAtField
		model.ResurrectTMinus1UsedAtField
		model.DeserializedUsersKey
		model.IDT0Field
		model.IDTMinus1Field
		model.BalanceTotalStandardField
		model.BalanceTotalPreStakingField
		model.BalanceTotalMintedField
		model.BalanceTotalSlashedField
		model.BalanceSoloPendingAppliedField
		model.BalanceT1PendingAppliedField
		model.BalanceT2PendingAppliedField
		model.BalanceSoloField
		model.BalanceT0Field
		model.BalanceT1Field
		model.BalanceT2Field
		model.BalanceForT0Field
		model.BalanceForTMinus1Field
		model.SlashingRateSoloField
		model.SlashingRateT0Field
		model.SlashingRateT1Field
		model.SlashingRateT2Field
		model.SlashingRateForT0Field
		model.SlashingRateForTMinus1Field
		model.ExtraBonusDaysClaimNotAvailableField
	}

	referralUpdated struct {
		model.DeserializedUsersKey
		model.IDT0Field
		model.IDTMinus1Field
	}

	referral struct {
		model.MiningSessionSoloStartedAtField
		model.MiningSessionSoloEndedAtField
		model.MiningSessionSoloPreviouslyEndedAtField
		model.ResurrectSoloUsedAtField
		model.UserIDField
		model.IDT0Field
		model.DeserializedUsersKey
	}

	recalculateReferral struct {
		model.BalanceForTMinus1Field
		model.UserIDField
		model.DeserializedUsersKey
	}

	recalculated struct {
		model.RecalculatedBalanceForTMinus1AtField
		model.DeserializedRecalculatedUsersKey
	}

	referralCountGuardUpdatedUser struct {
		model.ReferralsCountChangeGuardUpdatedAtField
		model.DeserializedUsersKey
	}

	referralThatStoppedMining struct {
		StoppedMiningAt     *time.Time
		ID, IDT0, IDTMinus1 int64
	}

	dryrunUser struct {
		model.DeserializedDryRunUsersKey
		model.IDTMinus1Field
		model.RecalculatedBalanceForTMinus1AtField
	}

	miner struct {
		mb                            messagebroker.Client
		db                            storage.DB
		dwhClient                     dwh.Client
		cancel                        context.CancelFunc
		telemetry                     *telemetry
		wg                            *sync.WaitGroup
		extraBonusStartDate           *time.Time
		dbPG                          *storagePG.DB
		extraBonusIndicesDistribution map[uint16]map[uint16]uint16
	}
	config struct {
		disableAdvancedTeam *atomic.Pointer[[]string]
		tokenomics.Config   `mapstructure:",squash"` //nolint:tagliatelle // Nope.
		Workers             int64                    `yaml:"workers"`
		BatchSize           int64                    `yaml:"batchSize"`
		Development         bool                     `yaml:"development"`
	}
)
