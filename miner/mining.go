// SPDX-License-Identifier: ice License 1.0

package miner

import (
	"github.com/ice-blockchain/freezer/tokenomics"
	"github.com/ice-blockchain/wintr/time"
)

func mine(baseMiningRate float64, now *time.Time, usr *user, t0Ref, tMinus1Ref *referral) (updatedUser, history *user) {
	if usr == nil || usr.MiningSessionSoloStartedAt.IsNil() || usr.MiningSessionSoloEndedAt.IsNil() {
		return nil, nil
	}
	clonedUser1 := *usr
	updatedUser = &clonedUser1
	resurrect(now, updatedUser, t0Ref, tMinus1Ref)
	changeT0AndTMinus1Referrals(updatedUser)
	if updatedUser.MiningSessionSoloEndedAt.Before(*now.Time) &&
		updatedUser.BalanceSolo == 0 &&
		updatedUser.BalanceT0 == 0 &&
		updatedUser.BalanceT1 == 0 &&
		updatedUser.BalanceT2 == 0 &&
		updatedUser.BalanceSoloPending-updatedUser.BalanceSoloPendingApplied == 0 &&
		updatedUser.BalanceForT0 == 0 &&
		updatedUser.BalanceForTMinus1 == 0 {
		if updatedUser.BalanceT1Pending-updatedUser.BalanceT1PendingApplied != 0 ||
			updatedUser.BalanceT2Pending-updatedUser.BalanceT2PendingApplied != 0 {
			updatedUser.BalanceT1PendingApplied = updatedUser.BalanceT1Pending
			updatedUser.BalanceT2PendingApplied = updatedUser.BalanceT2Pending

			return updatedUser, nil
		}

		return nil, nil
	}

	if updatedUser.BalanceLastUpdatedAt.IsNil() {
		updatedUser.BalanceLastUpdatedAt = updatedUser.MiningSessionSoloStartedAt
	} else if updatedUser.BalanceLastUpdatedAt.Year() != now.Year() ||
		updatedUser.BalanceLastUpdatedAt.YearDay() != now.YearDay() ||
		updatedUser.BalanceLastUpdatedAt.Hour() != now.Hour() ||
		(cfg.Development && updatedUser.BalanceLastUpdatedAt.Minute() != now.Minute()) {
		clonedUser2 := *usr
		history = &clonedUser2
		history.HistoryPart = historyPart(history.BalanceLastUpdatedAt)
		updatedUser.BalanceTotalSlashed = 0
		updatedUser.BalanceTotalMinted = 0
	}

	timeSpent := now.Sub(*updatedUser.BalanceLastUpdatedAt.Time)
	var mintedAmount float64

	if updatedUser.MiningSessionSoloEndedAt.After(*now.Time) {
		if !updatedUser.ExtraBonusStartedAt.IsNil() && now.Before(updatedUser.ExtraBonusStartedAt.Add(cfg.ExtraBonuses.Duration)) {
			rate := (100 + float64(updatedUser.ExtraBonus)) * baseMiningRate * timeSpent.Hours() / 100.
			updatedUser.BalanceSolo += rate
			mintedAmount += rate
		} else {
			rate := baseMiningRate * timeSpent.Hours()
			updatedUser.BalanceSolo += rate
			mintedAmount += rate
		}
		if t0Ref != nil && !t0Ref.MiningSessionSoloEndedAt.IsNil() && t0Ref.MiningSessionSoloEndedAt.After(*now.Time) {
			rate := 25 * baseMiningRate * timeSpent.Hours() / 100
			updatedUser.BalanceForT0 += rate
			updatedUser.BalanceT0 += rate
			mintedAmount += rate
		}
		if tMinus1Ref != nil && !tMinus1Ref.MiningSessionSoloEndedAt.IsNil() && tMinus1Ref.MiningSessionSoloEndedAt.After(*now.Time) {
			updatedUser.BalanceForTMinus1 += 5 * baseMiningRate * timeSpent.Hours() / 100
		}
		if updatedUser.ActiveT1Referrals < 0 {
			updatedUser.ActiveT1Referrals = 0
		}
		if updatedUser.ActiveT2Referrals < 0 {
			updatedUser.ActiveT2Referrals = 0
		}
		t1Rate := (25 * float64(updatedUser.ActiveT1Referrals)) * baseMiningRate * timeSpent.Hours() / 100
		t2Rate := (5 * float64(updatedUser.ActiveT2Referrals)) * baseMiningRate * timeSpent.Hours() / 100
		updatedUser.BalanceT1 += t1Rate
		updatedUser.BalanceT2 += t2Rate
		mintedAmount += t1Rate + t2Rate
	} else {
		if updatedUser.SlashingRateSolo == 0 {
			updatedUser.SlashingRateSolo = updatedUser.BalanceSolo / 60. / 24.
		}
		if updatedUser.SlashingRateT0 == 0 {
			updatedUser.SlashingRateT0 = updatedUser.BalanceT0 / 60. / 24.
		}
		if updatedUser.SlashingRateT1 == 0 {
			updatedUser.SlashingRateT1 = updatedUser.BalanceT1 / 60. / 24.
		}
		if updatedUser.SlashingRateT2 == 0 {
			updatedUser.SlashingRateT2 = updatedUser.BalanceT2 / 60. / 24.
		}
	}

	if t0Ref != nil &&
		!t0Ref.MiningSessionSoloEndedAt.IsNil() &&
		t0Ref.MiningSessionSoloEndedAt.Before(*now.Time) &&
		updatedUser.SlashingRateForT0 == 0 {
		updatedUser.SlashingRateForT0 = updatedUser.BalanceForT0 / 60. / 24.
	}

	if tMinus1Ref != nil &&
		!tMinus1Ref.MiningSessionSoloEndedAt.IsNil() &&
		tMinus1Ref.MiningSessionSoloEndedAt.Before(*now.Time) &&
		updatedUser.SlashingRateForTMinus1 == 0 {
		updatedUser.SlashingRateForTMinus1 = updatedUser.BalanceForTMinus1 / 60. / 24.
	}

	slashedAmount := (updatedUser.SlashingRateSolo + updatedUser.SlashingRateT0 + updatedUser.SlashingRateT1 + updatedUser.SlashingRateT2) * timeSpent.Hours()
	updatedUser.BalanceSolo -= updatedUser.SlashingRateSolo * timeSpent.Hours()
	updatedUser.BalanceForTMinus1 -= updatedUser.SlashingRateForTMinus1 * timeSpent.Hours()
	updatedUser.BalanceForT0 -= updatedUser.SlashingRateForT0 * timeSpent.Hours()
	updatedUser.BalanceT0 -= updatedUser.SlashingRateT0 * timeSpent.Hours()
	updatedUser.BalanceT1 -= updatedUser.SlashingRateT1 * timeSpent.Hours()
	updatedUser.BalanceT2 -= updatedUser.SlashingRateT2 * timeSpent.Hours()

	unAppliedSoloPending := updatedUser.BalanceSoloPending - updatedUser.BalanceSoloPendingApplied
	unAppliedT1Pending := updatedUser.BalanceT1Pending - updatedUser.BalanceT1PendingApplied
	unAppliedT2Pending := updatedUser.BalanceT2Pending - updatedUser.BalanceT2PendingApplied
	updatedUser.BalanceSoloPendingApplied = updatedUser.BalanceSoloPending
	updatedUser.BalanceT1PendingApplied = updatedUser.BalanceT1Pending
	updatedUser.BalanceT2PendingApplied = updatedUser.BalanceT2Pending

	updatedUser.BalanceSolo += unAppliedSoloPending
	updatedUser.BalanceT1 += unAppliedT1Pending
	updatedUser.BalanceT2 += unAppliedT2Pending

	if unAppliedSoloPending < 0 {
		slashedAmount += unAppliedSoloPending
	} else {
		mintedAmount += unAppliedSoloPending
	}
	if unAppliedT1Pending < 0 {
		slashedAmount += unAppliedT1Pending
	} else {
		mintedAmount += unAppliedT1Pending
	}
	if unAppliedT2Pending < 0 {
		slashedAmount += unAppliedT2Pending
	} else {
		mintedAmount += unAppliedT2Pending
	}
	if updatedUser.BalanceSolo < 0 {
		updatedUser.BalanceSolo = 0
	}
	if updatedUser.BalanceT0 < 0 {
		updatedUser.BalanceT0 = 0
	}
	if updatedUser.BalanceT1 < 0 {
		updatedUser.BalanceT1 = 0
	}
	if updatedUser.BalanceT2 < 0 {
		updatedUser.BalanceT2 = 0
	}
	if updatedUser.BalanceForT0 < 0 {
		updatedUser.BalanceForT0 = 0
	}
	if updatedUser.BalanceForTMinus1 < 0 {
		updatedUser.BalanceForTMinus1 = 0
	}
	if unAppliedSoloPending == 0 {
		updatedUser.BalanceSoloPendingApplied = 0
	}
	if unAppliedT1Pending == 0 {
		updatedUser.BalanceT1PendingApplied = 0
	}
	if unAppliedT2Pending == 0 {
		updatedUser.BalanceT2PendingApplied = 0
	}
	if usr.BalanceTotalPreStaking+usr.BalanceTotalStandard == 0 {
		slashedAmount = 0
	}

	totalAmount := updatedUser.BalanceSolo + updatedUser.BalanceT0 + updatedUser.BalanceT1 + updatedUser.BalanceT2
	updatedUser.BalanceTotalStandard, updatedUser.BalanceTotalPreStaking = tokenomics.ApplyPreStaking(totalAmount, updatedUser.PreStakingAllocation, updatedUser.PreStakingBonus)
	mintedStandard, mintedPreStaking := tokenomics.ApplyPreStaking(mintedAmount, updatedUser.PreStakingAllocation, updatedUser.PreStakingBonus)
	slashedStandard, slashedPreStaking := tokenomics.ApplyPreStaking(slashedAmount, updatedUser.PreStakingAllocation, updatedUser.PreStakingBonus)
	updatedUser.BalanceTotalMinted += mintedStandard + mintedPreStaking
	updatedUser.BalanceTotalSlashed += slashedStandard + slashedPreStaking
	updatedUser.BalanceLastUpdatedAt = now

	return updatedUser, history
}

func historyPart(date *time.Time) string {
	const hourFormat, minuteFormat = "2006-01-02T15", "2006-01-02T15:04"
	if cfg.Development {
		return date.Format(minuteFormat)
	} else {
		return date.Format(hourFormat)
	}
}