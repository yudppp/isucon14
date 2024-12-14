SET CHARACTER_SET_CLIENT = utf8mb4;
SET CHARACTER_SET_CONNECTION = utf8mb4;

USE isuride;

DROP TABLE IF EXISTS chair_metas;
CREATE TABLE chair_metas
(
  id                     VARCHAR(26) NOT NULL COMMENT '椅子ID',
  latitude               INTEGER     NOT NULL COMMENT '経度',
  longitude              INTEGER     NOT NULL COMMENT '緯度',
  is_moving              TINYINT(1)  NOT NULL COMMENT '配送中かどうか',
  total_distance         INTEGER     NOT NULL COMMENT '移動距離',
  total_distance_updated_at DATETIME(6) COMMENT '移動距離計測の最新更新日時',
  PRIMARY KEY (id)
) COMMENT = '椅子meta情報テーブル';

INSERT INTO chair_metas (id, latitude, longitude, is_moving, total_distance, total_distance_updated_at)
SELECT
    c.id,
    COALESCE(cl.latitude, 0) AS latitude,
    COALESCE(cl.longitude, 0) AS longitude,
    0 AS is_moving, -- 初期値として 0 を設定
    COALESCE(total_distance, 0) AS total_distance,
    COALESCE(total_distance_updated_at, NULL) AS total_distance_updated_at
FROM chairs c
LEFT JOIN (
    SELECT
        chair_id,
        SUM(IFNULL(distance, 0)) AS total_distance,
        MAX(created_at) AS total_distance_updated_at
    FROM (
        SELECT
            chair_id,
            created_at,
            ABS(latitude - LAG(latitude) OVER (PARTITION BY chair_id ORDER BY created_at)) +
            ABS(longitude - LAG(longitude) OVER (PARTITION BY chair_id ORDER BY created_at)) AS distance
        FROM chair_locations
    ) tmp
    GROUP BY chair_id
) distance_table ON distance_table.chair_id = c.id
LEFT JOIN (
    SELECT
        chair_id,
        latitude,
        longitude
    FROM chair_locations cl
    WHERE cl.created_at = (
        SELECT MAX(created_at)
        FROM chair_locations cl2
        WHERE cl2.chair_id = cl.chair_id
    )
) cl ON cl.chair_id = c.id;

