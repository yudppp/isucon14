USE isuride;

-- chairsテーブル
-- owner_idで絞り込むクエリ用
CREATE INDEX idx_chairs_owner_id ON chairs(owner_id);

-- 検討: access_tokenで直接検索する場合が多いなら
CREATE INDEX idx_chairs_access_token ON chairs(access_token);


-- ridesテーブル
-- chair_idとupdated_atでソート・検索するクエリ用
CREATE INDEX idx_rides_chairid_updatedat ON rides(chair_id, updated_at DESC);

-- user_idとcreated_atでユーザーのライド履歴を時系列で取得
CREATE INDEX idx_rides_userid_createdat ON rides(user_id, created_at DESC);

-- 検討: chair_idとcreated_atで並べ替えるクエリが多い場合
CREATE INDEX idx_rides_chairid_createdat ON rides(chair_id, created_at DESC);
CREATE INDEX idx_rides_chairid_createdat2 ON rides(chair_id, created_at ASC);

CREATE INDEX idx_chair_metas_id ON chair_metas(id);
CREATE INDEX idx_chair_metas_updatedat ON chair_metas(total_distance_updated_at);
CREATE INDEX idx_chair_metas_is_moving ON chair_metas(is_moving);

-- ride_statusesテーブル
-- ride_idとcreated_atでライドステータス履歴取得
CREATE INDEX idx_ride_statuses_rideid_createdat ON ride_statuses(ride_id, created_at);
CREATE INDEX idx_ride_statuses_rideid_createdat2 ON ride_statuses(ride_id, created_at DESC);

-- 検討: 特定のstatusで絞り込みつつ時系列で取得する場合
CREATE INDEX idx_ride_statuses_rideid_status_createdat ON ride_statuses(ride_id, status, created_at);

-- 検討: app_sent_atがNULLのレコードを取得する場合
CREATE INDEX idx_ride_statuses_rideid_appsentat_createdat ON ride_statuses(ride_id, app_sent_at, created_at);

CREATE INDEX idx_ride_statuses_rideid_chairsent ON ride_statuses(ride_id, chair_sent_at);

-- chair_locationsテーブル
-- chair_idとcreated_atで椅子位置履歴を時系列取得
CREATE INDEX idx_chair_locations_chairid_createdat ON chair_locations(chair_id, created_at);


-- couponsテーブル
CREATE INDEX idx_coupons_userid_code_usedby ON coupons(user_id, code, used_by);
CREATE INDEX idx_coupons_userid_usedby_createdat ON coupons(user_id, used_by, created_at);
CREATE INDEX idx_coupons_usedby ON coupons(used_by);
CREATE INDEX idx_coupons_code ON coupons(code);
 
 CREATE INDEX idx_users_access_token ON users(access_token);
 CREATE INDEX idx_users_invitation_code ON users(invitation_code);
 