package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"slices"
)

func internalGetMatchingNothing(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching() error {
	ctx := context.Background()
	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
	rides := []Ride{}
	if err := db.SelectContext(ctx, &rides, `SELECT * FROM rides WHERE chair_id IS NULL`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}

	var chairs []ChairWithLocationAndSpeed
	query := `SELECT c.id, cl.latitude, cl.longitude, cm.speed
FROM chairs c
LEFT JOIN chair_locations cl ON c.id = cl.chair_id
    AND cl.created_at = (
        SELECT MAX(cl2.created_at)
        FROM chair_locations cl2
        WHERE cl2.chair_id = c.id
    )
JOIN chair_models cm ON c.model = cm.name
WHERE c.is_active = TRUE
AND NOT EXISTS (
    SELECT 1
    FROM rides r
    JOIN ride_statuses rs ON rs.ride_id = r.id
    WHERE r.chair_id = c.id
    GROUP BY r.id
    HAVING COUNT(rs.chair_sent_at) != 6
)`
	if err := db.SelectContext(ctx, &chairs, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) || len(chairs) == 0 {
			return sql.ErrNoRows
		}
		return err
	}

	matchings := selectMatchings(rides, chairs)
	for _, matching := range matchings {
		if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matching.ChairID, matching.RideID); err != nil {
			return err
		}
	}
	return nil
}

func selectMatching(ride *Ride, chairs []ChairWithLocationAndSpeed) *ChairWithLocationAndSpeed {
	rideDistance := calculateDistance(ride.PickupLatitude, ride.PickupLongitude, ride.DestinationLatitude, ride.DestinationLongitude)
	minTargetTime := float64(0)
	var minChair *ChairWithLocationAndSpeed
	for _, chair := range chairs {
		// 一番早く着く椅子を選ぶ
		distance := rideDistance + calculateDistance(ride.PickupLatitude, ride.PickupLongitude, chair.Latitude, chair.Longitude)
		targetTime := float64(distance) / float64(chair.Speed)
		if minChair == nil || targetTime < minTargetTime {
			minTargetTime = targetTime
			minChair = &chair
		}
	}
	return minChair
}

type ChairMatching struct {
	RideID  string
	ChairID string
}

func selectMatchings(rides []Ride, chairs []ChairWithLocationAndSpeed) []ChairMatching {
	results := []ChairMatching{}
	matchedChair := map[string]bool{}
	slices.SortFunc(rides, func(a, b Ride) int {
		ad := calculateDistance(a.PickupLatitude, a.PickupLongitude, a.DestinationLatitude, a.DestinationLongitude)
		bd := calculateDistance(b.PickupLatitude, b.PickupLongitude, b.DestinationLatitude, b.DestinationLongitude)
		return ad - bd
	})
	for _, ride := range rides {
		rideDistance := calculateDistance(ride.PickupLatitude, ride.PickupLongitude, ride.DestinationLatitude, ride.DestinationLongitude)
		minTargetTime := float64(0)
		var minChair *ChairWithLocationAndSpeed
		for _, chair := range chairs {
			if _, exist := matchedChair[chair.ID]; exist {
				continue
			}
			// 一番早く着く椅子を選ぶ
			distance := rideDistance + calculateDistance(ride.PickupLatitude, ride.PickupLongitude, chair.Latitude, chair.Longitude)
			targetTime := float64(distance) / float64(chair.Speed)
			if minChair == nil || targetTime < minTargetTime {
				minTargetTime = targetTime
				minChair = &chair
			}
		}
		if minChair == nil {
			break
		}
		matchedChair[minChair.ID] = true
		results = append(results, ChairMatching{
			RideID:  ride.ID,
			ChairID: minChair.ID,
		})
	}
	return results
}
