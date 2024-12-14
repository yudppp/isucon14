package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"slices"

	"golang.org/x/sync/errgroup"
)

func internalGetMatchingNothing(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching() error {
	ctx := context.Background()
	rides := []Ride{}
	if err := db.SelectContext(ctx, &rides, `SELECT * FROM rides WHERE chair_id IS NULL`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}
	if len(rides) == 0 {
		return nil
	}
	var chairs []ChairWithLocationAndSpeed
	query := `SELECT c.id, cl.latitude, cl.longitude, cm.speed
FROM chairs c
LEFT JOIN chair_metas cl ON c.id = cl.id
JOIN chair_models cm ON c.model = cm.name
WHERE c.is_active = TRUE AND cl.is_moving = FALSE;
`
	if err := db.SelectContext(ctx, &chairs, query); err != nil {
		if errors.Is(err, sql.ErrNoRows) || len(chairs) == 0 {
			return sql.ErrNoRows
		}
		return err
	}
	if len(chairs) == 0 {
		return nil
	}
	ridesByRegion := map[RegionID][]Ride{}
	for _, ride := range rides {
		regionID := calculateRegionID(ride.PickupLatitude, ride.PickupLongitude)
		ridesByRegion[regionID] = append(ridesByRegion[regionID], ride)
	}
	chairsByRegion := map[RegionID][]ChairWithLocationAndSpeed{}
	for _, chair := range chairs {
		regionID := calculateRegionID(chair.Latitude, chair.Longitude)
		chairsByRegion[regionID] = append(chairsByRegion[regionID], chair)
	}
	// regionごとに並列で実行する
	eg, ctx := errgroup.WithContext(ctx)
	for _, regionID := range AllRegions {
		rides, ok := ridesByRegion[regionID]
		if !ok {
			continue
		}
		chairs, ok := chairsByRegion[regionID]
		if !ok {
			continue
		}
		eg.Go(func() error {
			matchings := selectMatchings(rides, chairs)
			tx, err := db.Begin()
			if err != nil {
				return err
			}
			for _, matching := range matchings {
				if _, err := tx.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matching.ChairID, matching.RideID); err != nil {
					return err
				}
				_, err = tx.ExecContext(
					ctx,
					`UPDATE chair_metas SET is_moving = TRUE WHERE id = ?`,
					matching.ChairID)
				if err != nil {
					return err
				}
			}
			return tx.Commit()
		})
	}
	return eg.Wait()
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
	matchedChair := make(map[string]bool, len(chairs))

	// 各ライドについて事前に目的地までの距離を計算しておく
	type rideWithDist struct {
		Ride         Ride
		RideDistance int
	}
	rideDistList := make([]rideWithDist, len(rides))
	for i, r := range rides {
		d := calculateDistance(r.PickupLatitude, r.PickupLongitude, r.DestinationLatitude, r.DestinationLongitude)
		rideDistList[i] = rideWithDist{Ride: r, RideDistance: d}
	}

	// rideDistanceでソート
	slices.SortFunc(rideDistList, func(a, b rideWithDist) int {
		return a.RideDistance - b.RideDistance
	})

	for _, rw := range rideDistList {
		ride := rw.Ride
		rideDistance := rw.RideDistance

		minTargetTime := float64(-1) // 初回比較用に-1をいれておく
		var minPickUpDistance int
		var minChairID string

		for _, chair := range chairs {
			if matchedChair[chair.ID] {
				continue
			}

			// 一番早く着く椅子を選ぶ
			pickUpDistance := calculateDistance(ride.PickupLatitude, ride.PickupLongitude, chair.Latitude, chair.Longitude)
			distance := rideDistance + pickUpDistance
			targetTime := float64(distance) / float64(chair.Speed)
			if minTargetTime < 0 || targetTime < minTargetTime {
				minTargetTime = targetTime
				minChairID = chair.ID
				minPickUpDistance = pickUpDistance
			}
		}

		if minChairID == "" {
			// 空いている椅子がない
			break
		}
		if minPickUpDistance > 100 {
			// 100以上離れている場合は無理に迎えにいかない
			break
		}

		matchedChair[minChairID] = true
		log.Printf("matched: %v %v %v %v %v", ride.ID, minChairID, ride.UserID, minPickUpDistance, rideDistance)
		results = append(results, ChairMatching{
			RideID:  ride.ID,
			ChairID: minChairID,
		})

		// 全椅子がマッチしてしまったら抜ける
		if len(matchedChair) == len(chairs) {
			break
		}
	}
	return results
}

type RegionID int

const (
	RegionA RegionID = iota + 1
	RegionB
)

var AllRegions = []RegionID{RegionA, RegionB}

func calculateRegionID(latitude, longitude int) RegionID {
	if latitude+longitude > 300 {
		return RegionB
	}
	return RegionA
}
