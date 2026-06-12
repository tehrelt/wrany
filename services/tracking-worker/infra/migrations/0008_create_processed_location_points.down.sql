ALTER TABLE trip_detection_state
    ALTER COLUMN late_arrival_window_sec SET DEFAULT 300;

ALTER TABLE trip_detection_state
    DROP COLUMN candidate_good_points;

DROP TABLE processed_location_points;
