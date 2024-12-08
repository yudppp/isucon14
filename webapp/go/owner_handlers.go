package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/oklog/ulid/v2"
)

const (
	initialFare     = 500
	farePerDistance = 100
)

type ownerPostOwnersRequest struct {
	Name string `json:"name"`
}

type ownerPostOwnersResponse struct {
	ID                 string `json:"id"`
	ChairRegisterToken string `json:"chair_register_token"`
}

func ownerPostOwners(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req := &ownerPostOwnersRequest{}
	if err := bindJSON(r, req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, errors.New("some of required fields(name) are empty"))
		return
	}

	ownerID := ulid.Make().String()
	accessToken := secureRandomStr(32)
	chairRegisterToken := secureRandomStr(32)

	_, err := db.ExecContext(
		ctx,
		"INSERT INTO owners (id, name, access_token, chair_register_token) VALUES (?, ?, ?, ?)",
		ownerID, req.Name, accessToken, chairRegisterToken,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Path:  "/",
		Name:  "owner_session",
		Value: accessToken,
	})

	writeJSON(w, http.StatusCreated, &ownerPostOwnersResponse{
		ID:                 ownerID,
		ChairRegisterToken: chairRegisterToken,
	})
}

type chairSales struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Sales int    `json:"sales"`
}

type modelSales struct {
	Model string `json:"model"`
	Sales int    `json:"sales"`
}

type ownerGetSalesResponse struct {
	TotalSales int          `json:"total_sales"`
	Chairs     []chairSales `json:"chairs"`
	Models     []modelSales `json:"models"`
}

func ownerGetSales(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	since := time.Unix(0, 0)
	until := time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC)
	if r.URL.Query().Get("since") != "" {
		parsed, err := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		since = time.UnixMilli(parsed)
	}
	if r.URL.Query().Get("until") != "" {
		parsed, err := strconv.ParseInt(r.URL.Query().Get("until"), 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		until = time.UnixMilli(parsed)
	}

	owner := r.Context().Value("owner").(*Owner)

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	// まずオーナーに紐づくchairsを取得
	chairs := []Chair{}
	if err := tx.SelectContext(ctx, &chairs, "SELECT * FROM chairs WHERE owner_id = ?", owner.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	if len(chairs) == 0 {
		// 椅子がない場合は合計0円で返す
		res := ownerGetSalesResponse{
			TotalSales: 0,
			Chairs:     []chairSales{},
			Models:     []modelSales{},
		}
		writeJSON(w, http.StatusOK, res)
		return
	}

	// chair_id一覧を抽出
	chairIDs := make([]string, 0, len(chairs))
	chairMap := make(map[string]*Chair, len(chairs))
	for i := range chairs {
		chairIDs = append(chairIDs, chairs[i].ID)
		chairMap[chairs[i].ID] = &chairs[i]
	}

	// ridesをまとめて集計するクエリ
	// status='COMPLETED'かつ指定期間内のライドが対象
	// 売上計算: 500 + 100*(abs(pickup_lat - dest_lat) + abs(pickup_lon - dest_lon))
	query, args, err := sqlx.In(`
		SELECT
			r.chair_id,
			SUM(500 + 100 * (ABS(r.pickup_latitude - r.destination_latitude) + ABS(r.pickup_longitude - r.destination_longitude))) AS total_sales
		FROM rides r
		JOIN ride_statuses rs ON r.id = rs.ride_id
		WHERE r.chair_id IN (?)
		  AND rs.status = 'COMPLETED'
		  AND r.updated_at BETWEEN ? AND ? + INTERVAL 999 MICROSECOND
		GROUP BY r.chair_id
	`, chairIDs, since, until)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	query = tx.Rebind(query)

	type ChairSalesAggregation struct {
		ChairID    string `db:"chair_id"`
		TotalSales int    `db:"total_sales"`
	}

	var aggResults []ChairSalesAggregation
	if err := tx.SelectContext(ctx, &aggResults, query, args...); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	// 結果をchair_id -> total_salesでまとめる
	totalSales := 0
	modelSalesByModel := map[string]int{}

	// 集計結果をmapで引けるようにする
	salesMap := make(map[string]int, len(aggResults))
	for _, a := range aggResults {
		salesMap[a.ChairID] = a.TotalSales
	}

	resChairs := []chairSales{}
	for _, chair := range chairs {
		sales := salesMap[chair.ID] // なければ0
		totalSales += sales
		resChairs = append(resChairs, chairSales{
			ID:    chair.ID,
			Name:  chair.Name,
			Sales: sales,
		})
		modelSalesByModel[chair.Model] += sales
	}

	models := make([]modelSales, 0, len(modelSalesByModel))
	for model, sales := range modelSalesByModel {
		models = append(models, modelSales{
			Model: model,
			Sales: sales,
		})
	}

	res := ownerGetSalesResponse{
		TotalSales: totalSales,
		Chairs:     resChairs,
		Models:     models,
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusOK, res)
}

func sumSales(rides []Ride) int {
	sale := 0
	for _, ride := range rides {
		sale += calculateSale(ride)
	}
	return sale
}

func calculateSale(ride Ride) int {
	return calculateFare(ride.PickupLatitude, ride.PickupLongitude, ride.DestinationLatitude, ride.DestinationLongitude)
}

type chairWithDetail struct {
	ID                     string       `db:"id"`
	OwnerID                string       `db:"owner_id"`
	Name                   string       `db:"name"`
	AccessToken            string       `db:"access_token"`
	Model                  string       `db:"model"`
	IsActive               bool         `db:"is_active"`
	CreatedAt              time.Time    `db:"created_at"`
	UpdatedAt              time.Time    `db:"updated_at"`
	TotalDistance          int          `db:"total_distance"`
	TotalDistanceUpdatedAt sql.NullTime `db:"total_distance_updated_at"`
}

type ownerGetChairResponse struct {
	Chairs []ownerGetChairResponseChair `json:"chairs"`
}

type ownerGetChairResponseChair struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	Model                  string `json:"model"`
	Active                 bool   `json:"active"`
	RegisteredAt           int64  `json:"registered_at"`
	TotalDistance          int    `json:"total_distance"`
	TotalDistanceUpdatedAt *int64 `json:"total_distance_updated_at,omitempty"`
}

func ownerGetChairs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	owner := ctx.Value("owner").(*Owner)

	chairs := []chairWithDetail{}
	if err := db.SelectContext(ctx, &chairs, `SELECT 
    chairs.id,
    chairs.owner_id,
    chairs.name,
    chairs.access_token,
    chairs.model,
    chairs.is_active,
    chairs.created_at,
    chairs.updated_at,
    IFNULL(chair_total_distances.total_distance, 0) AS total_distance,
    chair_total_distances.updated_at AS total_distance_updated_at
FROM 
    chairs
LEFT JOIN 
    chair_total_distances 
ON 
    chairs.id = chair_total_distances.chair_id
WHERE 
    chairs.owner_id = ?;
`, owner.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	res := ownerGetChairResponse{}
	for _, chair := range chairs {
		c := ownerGetChairResponseChair{
			ID:            chair.ID,
			Name:          chair.Name,
			Model:         chair.Model,
			Active:        chair.IsActive,
			RegisteredAt:  chair.CreatedAt.UnixMilli(),
			TotalDistance: chair.TotalDistance,
		}
		if chair.TotalDistanceUpdatedAt.Valid {
			t := chair.TotalDistanceUpdatedAt.Time.UnixMilli()
			c.TotalDistanceUpdatedAt = &t
		}
		res.Chairs = append(res.Chairs, c)
	}
	writeJSON(w, http.StatusOK, res)
}
