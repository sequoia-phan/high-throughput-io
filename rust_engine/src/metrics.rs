use lazy_static::lazy_static;
use prometheus::{Histogram, IntCounter, register_histogram, register_int_counter};

lazy_static! {
    pub static ref RUST_WRITE_BYTES_TOTAL: IntCounter = register_int_counter!(
        "rust_write_bytes_total",
        "Total log bytes written to the Rust buffered file writer"
    )
    .unwrap();
    pub static ref RUST_DISK_FLUSH_DURATION: Histogram = register_histogram!(
        "rust_disk_flush_duration_seconds",
        "Duration of flushing the Rust buffered log writer to disk"
    )
    .unwrap();
    pub static ref RUST_DISK_WRITE_FAILURES_TOTAL: IntCounter = register_int_counter!(
        "rust_disk_write_failures_total",
        "Total failed writes to the Rust buffered file writer"
    )
    .unwrap();
    pub static ref RUST_DISK_FLUSH_FAILURES_TOTAL: IntCounter = register_int_counter!(
        "rust_disk_flush_failures_total",
        "Total failed flushes of the Rust buffered file writer"
    )
    .unwrap();
    pub static ref RUST_INVALID_LOGS_TOTAL: IntCounter = register_int_counter!(
        "rust_invalid_logs_total",
        "Total invalid JSON log entries dropped by the Rust UDS listener"
    )
    .unwrap();
}
