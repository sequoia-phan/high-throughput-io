use std::{
    eprintln,
    error::Error,
    format,
    fs::{self},
    path::Path,
    println,
};

use chrono::Local;
use serde::Deserialize;
use serde_json::{Value, from_str};
use tokio::{
    fs::OpenOptions,
    io::{AsyncBufReadExt, AsyncWriteExt, BufReader, BufWriter},
    net::UnixListener,
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
                    }
                }

                // period
                _ = flush_interval.tick() => {
                    if let Err(e) = writer.flush().await {
                        eprintln!("[Disk Writer Error] Failed to flush buffer: {}", e);
                    }
                }
                else => break,
            }
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
