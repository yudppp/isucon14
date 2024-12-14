SET CHARACTER_SET_CLIENT = utf8mb4;
SET CHARACTER_SET_CONNECTION = utf8mb4;

USE isuride;

ALTER TABLE ride_statuses
ADD COLUMN chair_id VARCHAR(26) NULL COMMENT '椅子ID';

UPDATE ride_statuses rs
JOIN rides r ON rs.ride_id = r.id
SET rs.chair_id = r.chair_id;

CREATE INDEX idx_ride_statuses_chair_chair_sent_at_createdat ON ride_statuses(chair_id, chair_sent_at, created_at ASC);
CREATE INDEX idx_ride_statuses_chair_createdat_desc ON ride_statuses(chair_id, created_at DESC);