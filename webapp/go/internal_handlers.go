package main

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
)

func internalGetMatchingNothing(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching() error {
	ctx := context.Background()
	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
	ride := &Ride{}
	if err := db.GetContext(ctx, ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
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

	matched := selectMatching(ride, chairs)
	if matched == nil {
		return sql.ErrNoRows
	}

	if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matched.ID, ride.ID); err != nil {
		return err
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
