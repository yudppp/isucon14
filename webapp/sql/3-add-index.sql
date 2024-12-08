USE isuride;

-- chairsテーブル
-- owner_idで絞り込むクエリ用
CREATE INDEX idx_chairs_owner_id ON chairs(owner_id);

-- 検討: access_tokenで直接検索する場合が多いなら
CREATE INDEX idx_chairs_access_token ON chairs(access_token);


-- ridesテーブル
-- chair_idとupdated_atでソート・検索するクエリ用
CREATE INDEX idx_rides_chairid_updatedat ON rides(chair_id, updated_at);

-- user_idとcreated_atでユーザーのライド履歴を時系列で取得
CREATE INDEX idx_rides_userid_createdat ON rides(user_id, created_at);

-- 検討: chair_idとcreated_atで並べ替えるクエリが多い場合
CREATE INDEX idx_rides_chairid_createdat ON rides(chair_id, created_at);


-- ride_statusesテーブル
-- ride_idとcreated_atでライドステータス履歴取得
CREATE INDEX idx_ride_statuses_rideid_createdat ON ride_statuses(ride_id, created_at);

-- 検討: 特定のstatusで絞り込みつつ時系列で取得する場合
CREATE INDEX idx_ride_statuses_rideid_status_createdat ON ride_statuses(ride_id, status, created_at);

-- 検討: app_sent_atがNULLのレコードを取得する場合
CREATE INDEX idx_ride_statuses_rideid_appsentat_createdat ON ride_statuses(ride_id, app_sent_at, created_at);


-- chair_locationsテーブル
-- chair_idとcreated_atで椅子位置履歴を時系列取得
CREATE INDEX idx_chair_locations_chairid_createdat ON chair_locations(chair_id, created_at);


-- couponsテーブル
-- user_id, used_byで未使用クーポン等を絞り込む場合
CREATE INDEX idx_coupons_userid_usedby ON coupons(user_id, used_by);
