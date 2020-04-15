create table test_updates
    (
        at                  timestamptz     not null,
        vehicle_id          bigint          not null,
        latency_in_seconds  int             not null
    );
