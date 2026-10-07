CREATE TABLE IF NOT EXISTS trading.raw_ticks (
    timestamp      DateTime64(3),
    symbol         String,
    price          Float64,
    size           Float64,
    side           String,
    open_interest  Float64
) ENGINE = MergeTree()
ORDER BY (symbol, timestamp)
PARTITION BY toYYYYMM(timestamp)
TTL toDateTime(timestamp) + INTERVAL 6 MONTH;

CREATE TABLE IF NOT EXISTS trading.aggregated_bars (
    timestamp              DateTime64(3),
    timeframe              String,
    symbol                 String,
    open                   Float64,
    high                   Float64,
    low                    Float64,
    close                  Float64,
    volume                 Float64,
    buy_volume             Float64,
    sell_volume            Float64,
    buy_sell_volume_delta  Float64,
    vwap                   Float64,
    open_interest          Float64,
    tick_count             Int64,
    complete               UInt8,
    footprint              Array(Tuple(price Float64, buy_volume Float64, sell_volume Float64))
) ENGINE = MergeTree()
ORDER BY (symbol, timestamp)
PARTITION BY toYYYYMM(timestamp)
TTL toDateTime(timestamp) + INTERVAL 12 MONTH;
