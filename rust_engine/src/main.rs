use std::{
    eprintln,
    error::Error,
    format,
    fs::{self},
    path::Path,
    println,
};

mod metrics;

use chrono::Local;
use prometheus::{Encoder, TextEncoder};
use serde::Deserialize;
use serde_json::{Value, from_str};
use tokio::{
    fs::OpenOptions,
    io::{AsyncBufReadExt, AsyncWriteExt, BufReader, BufWriter},
    net::{TcpListener, UnixListener},
    sync::mpsc,
    time::{Duration, interval},
};

#[derive(Debug, Deserialize)]
struct LogEntry {
    client_id: String,
    timestamp: i64,
    level: String,
    message: String,
    #[serde(default)]
    payload: Option<Value>,
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    let socket_path = "/tmp/io_rust_engine.sock";
    let log_dir = "./logs";
    let metrics_addr = std::env::var("RUST_METRICS_ADDR")
        .unwrap_or_else(|_| "0.0.0.0:9090".to_string());

    lazy_static::initialize(&metrics::RUST_WRITE_BYTES_TOTAL);
    lazy_static::initialize(&metrics::RUST_DISK_FLUSH_DURATION);
    lazy_static::initialize(&metrics::RUST_DISK_WRITE_FAILURES_TOTAL);
    lazy_static::initialize(&metrics::RUST_DISK_FLUSH_FAILURES_TOTAL);
    lazy_static::initialize(&metrics::RUST_INVALID_LOGS_TOTAL);

    if !Path::new(log_dir).exists() {
        fs::create_dir_all(log_dir)?;
    }

    if Path::new(socket_path).exists() {
        fs::remove_file(socket_path)?;
    }

    let (tx, mut rx) = mpsc::channel::<String>(50_000);

    // ==========================================
    // TASK 1: BACKGROUND BUFFERED DISK WRITER
    // ==========================================
    tokio::spawn(async move {
        let date_str = Local::now().format("%Y-%m-%d").to_string();
        let log_file_path = format!("{}/app_{}.log", log_dir, date_str);

        let file = OpenOptions::new()
            .create(true)
            .append(true)
            .open(&log_file_path)
            .await
            .expect("Can't create or open file log");

        let mut writer = BufWriter::with_capacity(64 * 1024, file);
        let mut flush_interval = interval(Duration::from_secs(1));

        println!("[Rust Disk Writer] Ready writing to {}", log_file_path);

        loop {
            tokio::select! {
                Some(raw_log) = rx.recv() => {
                    if let Err(e) = writer.write_all(raw_log.as_bytes()).await {
                        eprintln!("[Disk Writer Error] Failed to write: {}", e);
                        metrics::RUST_DISK_WRITE_FAILURES_TOTAL.inc();
                    } else {
                        metrics::RUST_WRITE_BYTES_TOTAL.inc_by(raw_log.len() as u64);
                    }
                }

                // period
                _ = flush_interval.tick() => {
                    let flush_started = std::time::Instant::now();
                    if let Err(e) = writer.flush().await {
                        eprintln!("[Disk Writer Error] Failed to flush buffer: {}", e);
                        metrics::RUST_DISK_FLUSH_FAILURES_TOTAL.inc();
                    } else {
                        metrics::RUST_DISK_FLUSH_DURATION.observe(flush_started.elapsed().as_secs_f64());
                    }
                }
                else => break,
            }
        }
    });

    tokio::spawn(async move {
        if let Err(e) = serve_metrics(&metrics_addr).await {
            eprintln!("[Rust Metrics Error] Failed to serve metrics: {}", e);
        }
    });

    // ==========================================
    // TASK 2: UDS SOCKET LISTENER
    // ==========================================
    let listener = UnixListener::bind(socket_path)?;
    println!("[Rust Engine] Listening on UDS: {}", socket_path);

    loop {
        match listener.accept().await {
            Ok((stream, _)) => {
                let tx_clone = tx.clone();
                tokio::spawn(async move {
                    let mut reader = BufReader::new(stream);
                    let mut line = String::new();

                    while let Ok(bytes_read) = reader.read_line(&mut line).await {
                        if bytes_read == 0 {
                            break;
                        }

                        // Validate JSON cơ bản trước khi đẩy qua Writer Channel
                        if serde_json::from_str::<LogEntry>(&line).is_ok() {
                            if let Err(e) = tx_clone.send(line.clone()).await {
                                eprintln!("[Rust Channel Error] Disk writer channel closed: {}", e);
                                break;
                            }
                        } else {
                            eprintln!("[Rust Error] Invalid JSON payload dropped");
                            metrics::RUST_INVALID_LOGS_TOTAL.inc();
                        }

                        line.clear();
                    }
                });
            }
            Err(e) => {
                eprintln!("[Rust Error] Failed to accept connection: {}", e);
            }
        }
    }
}

async fn serve_metrics(addr: &str) -> Result<(), Box<dyn Error>> {
    let listener = TcpListener::bind(addr).await?;
    println!("[Rust Metrics] Listening on http://{}/metrics", addr);

    loop {
        let (mut stream, _) = listener.accept().await?;
        let metric_families = prometheus::gather();
        let mut body = Vec::new();
        let encoder = TextEncoder::new();

        if let Err(e) = encoder.encode(&metric_families, &mut body) {
            eprintln!("[Rust Metrics Error] Failed to encode metrics: {}", e);
            continue;
        }

        let headers = format!(
            "HTTP/1.1 200 OK\r\nContent-Type: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            encoder.format_type(),
            body.len(),
        );

        if stream.write_all(headers.as_bytes()).await.is_ok() {
            let _ = stream.write_all(&body).await;
        }
    }
}
