// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

use std::collections::{BTreeMap, HashMap, VecDeque};
use std::env;
use std::fmt;
use std::io::{self, BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::process::Command;
use std::sync::atomic::{AtomicBool, AtomicU32, Ordering};
use std::sync::{mpsc, Arc, Condvar, Mutex};
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

/// Parsed service port specification (`--ports=8080:http-api,9090:metrics`).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PortSpec {
    pub port: i32,
    pub name: String,
}

impl fmt::Display for PortSpec {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{{{} {}}}", self.port, self.name)
    }
}

/// CLI configuration for `devx-bridge-agent`.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct AgentFlags {
    pub control_port: i32,
    pub health_port: i32,
    pub ports: String,
}

impl Default for AgentFlags {
    fn default() -> Self {
        Self {
            control_port: 4200,
            health_port: 4201,
            ports: String::new(),
        }
    }
}

/// Formats a log message with the `[devx-bridge-agent] HH:MM:SS.ffffff ` prefix.
pub fn format_log_line(msg: &str) -> String {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default();
    let total_secs = now.as_secs();
    let micros = now.subsec_micros();
    let secs_of_day = total_secs % 86_400;
    let hours = secs_of_day / 3600;
    let minutes = (secs_of_day % 3600) / 60;
    let seconds = secs_of_day % 60;
    format!(
        "[devx-bridge-agent] {:02}:{:02}:{:02}.{:06} {}",
        hours, minutes, seconds, micros, msg
    )
}

pub fn log_info(msg: &str) {
    eprintln!("{}", format_log_line(msg));
}

/// Parses `--control-port`, `--health-port`, and `--ports` flags (supporting `-flag`, `--flag`, `=`, and space-separated values).
pub fn parse_flags(args: &[String]) -> Result<AgentFlags, String> {
    let mut cfg = AgentFlags::default();
    let mut i = 1;
    while i < args.len() {
        let arg = &args[i];
        if arg == "--" {
            break;
        }
        let stripped = if let Some(s) = arg.strip_prefix("--") {
            s
        } else if let Some(s) = arg.strip_prefix('-') {
            s
        } else {
            return Err(format!("unexpected positional argument: {arg}"));
        };

        let (key, val) = if let Some((k, v)) = stripped.split_once('=') {
            (k, v.to_string())
        } else {
            if stripped == "h" || stripped == "help" {
                return Err(
                    "Usage of devx-bridge-agent:\n  -control-port int\n    \tYamux control port (default 4200)\n  -health-port int\n    \tHealth check port (default 4201)\n  -ports string\n    \tService ports to mirror: port:name,port:name"
                        .to_string(),
                );
            }
            if i + 1 >= args.len() {
                return Err(format!("flag needs an argument: -{stripped}"));
            }
            i += 1;
            (stripped, args[i].clone())
        };

        match key {
            "control-port" => {
                cfg.control_port = val
                    .parse::<i32>()
                    .map_err(|e| format!("invalid value {val:?} for flag -control-port: {e}"))?;
            }
            "health-port" => {
                cfg.health_port = val
                    .parse::<i32>()
                    .map_err(|e| format!("invalid value {val:?} for flag -health-port: {e}"))?;
            }
            "ports" => {
                cfg.ports = val;
            }
            other => {
                return Err(format!("flag provided but not defined: -{other}"));
            }
        }
        i += 1;
    }
    Ok(cfg)
}

/// Parses `"8080:http-api,9090:metrics"` into port specs, matching Go's `parsePorts`.
pub fn parse_ports(raw: &str) -> Vec<PortSpec> {
    if raw.is_empty() {
        return Vec::new();
    }

    let mut specs = Vec::new();
    for item in raw.split(',') {
        let mut parts = item.splitn(2, ':');
        let port_str = parts.next().unwrap_or("");
        let Ok(port) = port_str.parse::<i32>() else {
            continue;
        };
        let name = parts.next().unwrap_or("").to_string();
        specs.push(PortSpec { port, name });
    }
    specs
}

/// Builds the JSON merge patch used by `restore_selector` to restore the original Service selector.
/// Uses `BTreeMap<String, String>` so keys are emitted in deterministic sorted order, matching Go's `encoding/json`.
pub fn build_restore_selector_patch(selector_json: &str) -> Result<String, String> {
    let selector: BTreeMap<String, String> = serde_json::from_str(selector_json)
        .map_err(|e| format!("self-healing: failed to parse original selector: {e}"))?;
    let selector_patch = serde_json::to_string(&selector)
        .map_err(|e| format!("self-healing: failed to serialize selector: {e}"))?;
    Ok(format!(
        r#"{{"metadata":{{"annotations":{{"devx-bridge-session":null}}}},"spec":{{"selector":{}}}}}"#,
        selector_patch
    ))
}

/// Trait abstracting `kubectl` execution so unit tests can verify exact command invocation.
pub trait CommandRunner {
    fn run_combined(&self, program: &str, args: &[&str]) -> Result<String, String>;
}

pub struct ProcessCommandRunner;

impl CommandRunner for ProcessCommandRunner {
    fn run_combined(&self, program: &str, args: &[&str]) -> Result<String, String> {
        let output = Command::new(program)
            .args(args)
            .output()
            .map_err(|e| e.to_string())?;
        let mut combined = String::from_utf8_lossy(&output.stdout).into_owned();
        combined.push_str(&String::from_utf8_lossy(&output.stderr));
        if output.status.success() {
            Ok(combined)
        } else {
            Err(format!(
                "{}: exit status {}",
                combined.trim_end(),
                output.status.code().unwrap_or(1)
            ))
        }
    }
}

/// Self-healing mechanism: restores the original Service selector via `kubectl patch`.
pub fn restore_selector_with_runner(
    selector_json: &str,
    service: &str,
    namespace: &str,
    runner: &dyn CommandRunner,
) -> Result<(), String> {
    if selector_json.is_empty() || service.is_empty() || namespace.is_empty() {
        log_info("self-healing skipped: missing selector/service/namespace env vars");
        return Ok(());
    }

    let patch = match build_restore_selector_patch(selector_json) {
        Ok(p) => p,
        Err(msg) => {
            log_info(&msg);
            return Err(msg);
        }
    };

    log_info(&format!(
        "self-healing: restoring selector for {namespace}/{service}"
    ));

    let args = [
        "patch",
        "service",
        service,
        "-n",
        namespace,
        "--type=merge",
        "-p",
        &patch,
    ];
    match runner.run_combined("kubectl", &args) {
        Ok(_) => {
            log_info(&format!(
                "self-healing successful: selector restored for {namespace}/{service}"
            ));
            Ok(())
        }
        Err(err) => {
            let msg = format!("self-healing failed: {err}");
            log_info(&msg);
            Err(msg)
        }
    }
}

pub fn restore_selector(selector_json: &str, service: &str, namespace: &str) {
    let _ = restore_selector_with_runner(selector_json, service, namespace, &ProcessCommandRunner);
}

/// Handles a single HTTP request on the health server, returning `(status_code, body, is_head)`.
pub fn route_health_request(request_line: &str) -> (u16, &'static str, bool) {
    let parts: Vec<&str> = request_line.split_whitespace().collect();
    if parts.len() < 2 {
        return (400, "400 bad request\n", false);
    }
    let method = parts[0];
    let raw_target = parts[1];
    let path = raw_target.split('?').next().unwrap_or(raw_target);
    let is_head = method.eq_ignore_ascii_case("HEAD");
    if path == "/healthz" {
        (200, "ok", is_head)
    } else {
        (404, "404 page not found\n", is_head)
    }
}

/// Serves a single HTTP connection for the health server.
pub fn handle_health_conn<S: Read + Write>(stream: S) -> io::Result<()> {
    let mut reader = BufReader::new(stream);
    let mut request_line = String::new();
    let n = reader.read_line(&mut request_line)?;
    if n == 0 {
        return Ok(());
    }
    // Drain headers until empty line
    loop {
        let mut line = String::new();
        let read = reader.read_line(&mut line)?;
        if read == 0 || line == "\r\n" || line == "\n" {
            break;
        }
    }

    let (status, body, is_head) = route_health_request(&request_line);
    let reason = match status {
        200 => "OK",
        400 => "Bad Request",
        _ => "Not Found",
    };

    let mut inner = reader.into_inner();
    let header = format!(
        "HTTP/1.1 {status} {reason}\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    );
    inner.write_all(header.as_bytes())?;
    if !is_head {
        inner.write_all(body.as_bytes())?;
    }
    inner.flush()
}

/// Runs the HTTP health server on the given `TcpListener` until `shutdown` is set or the listener closes.
pub fn serve_health_listener(listener: TcpListener, shutdown: Arc<AtomicBool>) {
    for incoming in listener.incoming() {
        if shutdown.load(Ordering::SeqCst) {
            break;
        }
        match incoming {
            Ok(conn) => {
                let _ = handle_health_conn(conn);
            }
            Err(_) => {
                if shutdown.load(Ordering::SeqCst) {
                    break;
                }
            }
        }
    }
}

/// Starts the health server on `0.0.0.0:<port>`.
pub fn start_health_server(port: i32, shutdown: Arc<AtomicBool>) {
    let addr = format!("0.0.0.0:{port}");
    match TcpListener::bind(&addr) {
        Ok(listener) => {
            log_info(&format!("health server on :{port}/healthz"));
            serve_health_listener(listener, shutdown);
        }
        Err(e) => {
            log_info(&format!("health server error: {e}"));
        }
    }
}

/// Listens on a service TCP port and sends accepted connections into `inbound`.
pub fn listen_service_port_on(
    listener: TcpListener,
    port: i32,
    inbound: mpsc::SyncSender<TcpStream>,
    shutdown: Arc<AtomicBool>,
) {
    log_info(&format!("listening on service port :{port}"));
    for conn in listener.incoming() {
        if shutdown.load(Ordering::SeqCst) {
            return;
        }
        match conn {
            Ok(stream) => {
                if inbound.send(stream).is_err() {
                    return;
                }
            }
            Err(_) => {
                if shutdown.load(Ordering::SeqCst) {
                    return;
                }
                continue;
            }
        }
    }
}

pub fn listen_service_port(
    port: i32,
    inbound: mpsc::SyncSender<TcpStream>,
    shutdown: Arc<AtomicBool>,
) {
    let addr = format!("0.0.0.0:{port}");
    match TcpListener::bind(&addr) {
        Ok(listener) => listen_service_port_on(listener, port, inbound, shutdown),
        Err(e) => log_info(&format!("failed to listen on service port {port}: {e}")),
    }
}

// ---------------------------------------------------------------------------
// HashiCorp Yamux v0 wire protocol implementation
// ---------------------------------------------------------------------------

pub const YAMUX_VERSION: u8 = 0;

pub const YAMUX_TYPE_DATA: u8 = 0;
pub const YAMUX_TYPE_WINDOW_UPDATE: u8 = 1;
pub const YAMUX_TYPE_PING: u8 = 2;
pub const YAMUX_TYPE_GO_AWAY: u8 = 3;

pub const YAMUX_FLAG_SYN: u16 = 0x0001;
pub const YAMUX_FLAG_ACK: u16 = 0x0002;
pub const YAMUX_FLAG_FIN: u16 = 0x0004;
pub const YAMUX_FLAG_RST: u16 = 0x0008;

pub const YAMUX_INITIAL_WINDOW: u32 = 256 * 1024;
pub const YAMUX_HEADER_LEN: usize = 12;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct YamuxHeader {
    pub version: u8,
    pub msg_type: u8,
    pub flags: u16,
    pub stream_id: u32,
    pub length: u32,
}

impl YamuxHeader {
    pub fn encode(&self) -> [u8; YAMUX_HEADER_LEN] {
        let mut buf = [0u8; YAMUX_HEADER_LEN];
        buf[0] = self.version;
        buf[1] = self.msg_type;
        buf[2..4].copy_from_slice(&self.flags.to_be_bytes());
        buf[4..8].copy_from_slice(&self.stream_id.to_be_bytes());
        buf[8..12].copy_from_slice(&self.length.to_be_bytes());
        buf
    }

    pub fn decode(buf: &[u8; YAMUX_HEADER_LEN]) -> Self {
        Self {
            version: buf[0],
            msg_type: buf[1],
            flags: u16::from_be_bytes([buf[2], buf[3]]),
            stream_id: u32::from_be_bytes([buf[4], buf[5], buf[6], buf[7]]),
            length: u32::from_be_bytes([buf[8], buf[9], buf[10], buf[11]]),
        }
    }
}

struct StreamSharedState {
    recv_buf: VecDeque<u8>,
    remote_closed: bool,
    local_closed: bool,
    reset: bool,
    send_window: u32,
}

struct StreamState {
    mutex: Mutex<StreamSharedState>,
    cvar: Condvar,
}

impl StreamState {
    fn new() -> Arc<Self> {
        Arc::new(Self {
            mutex: Mutex::new(StreamSharedState {
                recv_buf: VecDeque::new(),
                remote_closed: false,
                local_closed: false,
                reset: false,
                send_window: YAMUX_INITIAL_WINDOW,
            }),
            cvar: Condvar::new(),
        })
    }
}

/// A HashiCorp Yamux session operating over a bidirectional stream (e.g., `TcpStream`).
pub struct YamuxSession {
    writer: Mutex<Box<dyn Write + Send>>,
    streams: Mutex<HashMap<u32, Arc<StreamState>>>,
    next_stream_id: AtomicU32,
    closed: AtomicBool,
}

impl YamuxSession {
    /// Creates a new Yamux Server session (server-initiated streams use even IDs: 2, 4, 6, ...).
    pub fn server(conn: TcpStream) -> io::Result<Arc<Self>> {
        let reader_conn = conn.try_clone()?;
        Ok(Self::from_io(reader_conn, Box::new(conn), true))
    }

    /// Creates a Yamux session from any `Read` + `Write` pair.
    /// `is_server = true` starts stream IDs at 2 (even); `is_server = false` starts at 1 (odd).
    pub fn from_io<R: Read + Send + 'static>(
        reader: R,
        writer: Box<dyn Write + Send>,
        is_server: bool,
    ) -> Arc<Self> {
        let initial_id = if is_server { 2 } else { 1 };
        let session = Arc::new(Self {
            writer: Mutex::new(writer),
            streams: Mutex::new(HashMap::new()),
            next_stream_id: AtomicU32::new(initial_id),
            closed: AtomicBool::new(false),
        });

        let session_bg = Arc::clone(&session);
        thread::spawn(move || {
            session_bg.read_loop(reader);
        });

        session
    }

    pub fn is_closed(&self) -> bool {
        self.closed.load(Ordering::SeqCst)
    }

    fn write_frame(&self, header: YamuxHeader, payload: &[u8]) -> io::Result<()> {
        if self.is_closed() {
            return Err(io::Error::new(
                io::ErrorKind::BrokenPipe,
                "yamux session closed",
            ));
        }
        let mut w = self.writer.lock().unwrap();
        let hdr_bytes = header.encode();
        w.write_all(&hdr_bytes)?;
        if !payload.is_empty() {
            w.write_all(payload)?;
        }
        w.flush()
    }

    /// Opens a new outbound Yamux stream.
    pub fn open_stream(self: &Arc<Self>) -> io::Result<YamuxStream> {
        if self.is_closed() {
            return Err(io::Error::new(
                io::ErrorKind::BrokenPipe,
                "yamux session closed",
            ));
        }

        let stream_id = self.next_stream_id.fetch_add(2, Ordering::SeqCst);
        let state = StreamState::new();
        {
            let mut map = self.streams.lock().unwrap();
            map.insert(stream_id, Arc::clone(&state));
        }

        let syn_hdr = YamuxHeader {
            version: YAMUX_VERSION,
            msg_type: YAMUX_TYPE_WINDOW_UPDATE,
            flags: YAMUX_FLAG_SYN,
            stream_id,
            length: 0,
        };
        if let Err(e) = self.write_frame(syn_hdr, &[]) {
            self.streams.lock().unwrap().remove(&stream_id);
            return Err(e);
        }

        Ok(YamuxStream {
            session: Arc::clone(self),
            stream_id,
            state,
        })
    }

    /// Sends a Ping request frame (`SYN`) with the given ping ID.
    pub fn send_ping(&self, ping_id: u32) -> io::Result<()> {
        let hdr = YamuxHeader {
            version: YAMUX_VERSION,
            msg_type: YAMUX_TYPE_PING,
            flags: YAMUX_FLAG_SYN,
            stream_id: 0,
            length: ping_id,
        };
        self.write_frame(hdr, &[])
    }

    /// Closes the Yamux session, sending a normal GoAway frame (`code = 0`).
    pub fn close(&self) {
        if !self.closed.swap(true, Ordering::SeqCst) {
            let hdr = YamuxHeader {
                version: YAMUX_VERSION,
                msg_type: YAMUX_TYPE_GO_AWAY,
                flags: 0,
                stream_id: 0,
                length: 0,
            };
            if let Ok(mut w) = self.writer.lock() {
                let _ = w.write_all(&hdr.encode());
                let _ = w.flush();
            }
            let map = self.streams.lock().unwrap();
            for st in map.values() {
                let mut guard = st.mutex.lock().unwrap();
                guard.remote_closed = true;
                st.cvar.notify_all();
            }
        }
    }

    fn read_loop<R: Read>(&self, mut reader: R) {
        let mut hdr_buf = [0u8; YAMUX_HEADER_LEN];
        loop {
            if reader.read_exact(&mut hdr_buf).is_err() {
                break;
            }
            let hdr = YamuxHeader::decode(&hdr_buf);
            if hdr.version != YAMUX_VERSION {
                break;
            }

            match hdr.msg_type {
                YAMUX_TYPE_DATA => {
                    let mut payload = vec![0u8; hdr.length as usize];
                    if hdr.length > 0 && reader.read_exact(&mut payload).is_err() {
                        break;
                    }
                    self.handle_stream_frame(hdr, &payload);
                }
                YAMUX_TYPE_WINDOW_UPDATE => {
                    self.handle_stream_frame(hdr, &[]);
                }
                YAMUX_TYPE_PING => {
                    if hdr.flags & YAMUX_FLAG_SYN != 0 {
                        let pong = YamuxHeader {
                            version: YAMUX_VERSION,
                            msg_type: YAMUX_TYPE_PING,
                            flags: YAMUX_FLAG_ACK,
                            stream_id: 0,
                            length: hdr.length,
                        };
                        let _ = self.write_frame(pong, &[]);
                    }
                }
                YAMUX_TYPE_GO_AWAY => {
                    break;
                }
                _ => {
                    break;
                }
            }
        }

        self.closed.store(true, Ordering::SeqCst);
        let map = self.streams.lock().unwrap();
        for st in map.values() {
            let mut guard = st.mutex.lock().unwrap();
            guard.remote_closed = true;
            st.cvar.notify_all();
        }
    }

    fn handle_stream_frame(&self, hdr: YamuxHeader, payload: &[u8]) {
        let state = {
            let map = self.streams.lock().unwrap();
            map.get(&hdr.stream_id).cloned()
        };
        let Some(state) = state else {
            return;
        };

        let mut guard = state.mutex.lock().unwrap();
        if hdr.msg_type == YAMUX_TYPE_WINDOW_UPDATE && hdr.length > 0 {
            guard.send_window = guard.send_window.saturating_add(hdr.length);
        }
        if !payload.is_empty() {
            guard.recv_buf.extend(payload);
        }
        if hdr.flags & YAMUX_FLAG_RST != 0 {
            guard.reset = true;
            guard.remote_closed = true;
        }
        if hdr.flags & YAMUX_FLAG_FIN != 0 {
            guard.remote_closed = true;
        }
        state.cvar.notify_all();
    }
}

/// A multiplexed stream within a `YamuxSession`.
#[derive(Clone)]
pub struct YamuxStream {
    session: Arc<YamuxSession>,
    stream_id: u32,
    state: Arc<StreamState>,
}

impl YamuxStream {
    pub fn stream_id(&self) -> u32 {
        self.stream_id
    }

    /// Sends a FIN frame to close the stream if not already closed.
    pub fn close(&self) -> io::Result<()> {
        {
            let mut guard = self.state.mutex.lock().unwrap();
            if guard.local_closed {
                return Ok(());
            }
            guard.local_closed = true;
            self.state.cvar.notify_all();
        }
        let fin_hdr = YamuxHeader {
            version: YAMUX_VERSION,
            msg_type: YAMUX_TYPE_WINDOW_UPDATE,
            flags: YAMUX_FLAG_FIN,
            stream_id: self.stream_id,
            length: 0,
        };
        let res = self.session.write_frame(fin_hdr, &[]);
        let should_remove = {
            let guard = self.state.mutex.lock().unwrap();
            guard.remote_closed
        };
        if should_remove {
            self.session.streams.lock().unwrap().remove(&self.stream_id);
        }
        res
    }
}

impl Read for YamuxStream {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        if buf.is_empty() {
            return Ok(0);
        }
        let consumed = {
            let mut guard = self.state.mutex.lock().unwrap();
            while guard.recv_buf.is_empty() && !guard.remote_closed && !self.session.is_closed() {
                guard = self.state.cvar.wait(guard).unwrap();
            }
            if guard.reset && guard.recv_buf.is_empty() {
                return Err(io::Error::new(
                    io::ErrorKind::ConnectionReset,
                    "yamux stream reset",
                ));
            }
            if guard.recv_buf.is_empty() {
                return Ok(0);
            }
            let n = buf.len().min(guard.recv_buf.len());
            for b in buf.iter_mut().take(n) {
                *b = guard.recv_buf.pop_front().unwrap();
            }
            n as u32
        };

        // Grant window credit back to the remote sender for consumed bytes
        if consumed > 0 && !self.session.is_closed() {
            let wu_hdr = YamuxHeader {
                version: YAMUX_VERSION,
                msg_type: YAMUX_TYPE_WINDOW_UPDATE,
                flags: 0,
                stream_id: self.stream_id,
                length: consumed,
            };
            let _ = self.session.write_frame(wu_hdr, &[]);
        }

        Ok(consumed as usize)
    }
}

impl Write for YamuxStream {
    fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
        if buf.is_empty() {
            return Ok(0);
        }
        let chunk_len = {
            let mut guard = self.state.mutex.lock().unwrap();
            while guard.send_window == 0
                && !guard.local_closed
                && !guard.reset
                && !self.session.is_closed()
            {
                guard = self.state.cvar.wait(guard).unwrap();
            }
            if guard.local_closed || guard.reset || self.session.is_closed() {
                return Err(io::Error::new(
                    io::ErrorKind::BrokenPipe,
                    "yamux stream closed",
                ));
            }
            let n = (buf.len() as u32).min(guard.send_window).min(64 * 1024);
            guard.send_window -= n;
            n as usize
        };

        let hdr = YamuxHeader {
            version: YAMUX_VERSION,
            msg_type: YAMUX_TYPE_DATA,
            flags: 0,
            stream_id: self.stream_id,
            length: chunk_len as u32,
        };
        self.session.write_frame(hdr, &buf[..chunk_len])?;
        Ok(chunk_len)
    }

    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

/// Proxies an inbound TCP connection through a new Yamux stream on `session`.
pub fn proxy_to_stream(session: Arc<YamuxSession>, client_conn: TcpStream) {
    let stream = match session.open_stream() {
        Ok(s) => s,
        Err(e) => {
            log_info(&format!("failed to open Yamux stream: {e}"));
            return;
        }
    };

    let Ok(mut client_reader) = client_conn.try_clone() else {
        let _ = stream.close();
        return;
    };
    let mut client_writer = client_conn;

    let mut stream_writer = stream.clone();
    let mut stream_reader = stream.clone();

    let t1 = thread::spawn(move || {
        let _ = io::copy(&mut client_reader, &mut stream_writer);
    });
    let t2 = thread::spawn(move || {
        let _ = io::copy(&mut stream_reader, &mut client_writer);
    });

    let _ = t1.join();
    let _ = t2.join();
    let _ = stream.close();
}

/// Manages the Yamux server session with the `devx` CLI.
pub fn handle_cli_session(
    conn: TcpStream,
    inbound: &mpsc::Receiver<TcpStream>,
    shutdown: &Arc<AtomicBool>,
) {
    let session = match YamuxSession::server(conn) {
        Ok(s) => s,
        Err(e) => {
            log_info(&format!("failed to create Yamux server session: {e}"));
            return;
        }
    };

    log_info("Yamux session established with CLI");

    // Periodic keepalive ping thread (10s interval matching Go yamuxCfg.KeepAliveInterval)
    let session_ka = Arc::clone(&session);
    let shutdown_ka = Arc::clone(shutdown);
    thread::spawn(move || {
        let mut ping_id = 1u32;
        while !shutdown_ka.load(Ordering::SeqCst) && !session_ka.is_closed() {
            thread::sleep(Duration::from_secs(10));
            if shutdown_ka.load(Ordering::SeqCst) || session_ka.is_closed() {
                break;
            }
            if session_ka.send_ping(ping_id).is_err() {
                session_ka.close();
                break;
            }
            ping_id = ping_id.wrapping_add(1);
        }
    });

    loop {
        if shutdown.load(Ordering::SeqCst) || session.is_closed() {
            break;
        }
        match inbound.recv_timeout(Duration::from_millis(100)) {
            Ok(client_conn) => {
                if session.is_closed() {
                    log_info("Yamux session closed — dropping inbound connection");
                    drop(client_conn);
                    break;
                }
                let sess = Arc::clone(&session);
                thread::spawn(move || proxy_to_stream(sess, client_conn));
            }
            Err(mpsc::RecvTimeoutError::Timeout) => continue,
            Err(mpsc::RecvTimeoutError::Disconnected) => break,
        }
    }

    session.close();
}

/// Entrypoint for `devx-bridge-agent`.
pub fn run(args: &[String]) -> i32 {
    let flags = match parse_flags(args) {
        Ok(f) => f,
        Err(msg) => {
            eprintln!("{msg}");
            return 2;
        }
    };

    let original_selector = env::var("DEVX_ORIGINAL_SELECTOR").unwrap_or_default();
    let target_service = env::var("DEVX_TARGET_SERVICE").unwrap_or_default();
    let target_namespace = env::var("DEVX_TARGET_NAMESPACE").unwrap_or_default();

    let ports = parse_ports(&flags.ports);
    if ports.is_empty() {
        log_info("no service ports specified — use --ports=8080:name,9090:name");
        return 1;
    }

    let formatted_ports: Vec<String> = ports.iter().map(|p| p.to_string()).collect();
    log_info(&format!(
        "starting agent: control-port={} health-port={} ports=[{}]",
        flags.control_port,
        flags.health_port,
        formatted_ports.join(" ")
    ));

    let shutdown = Arc::new(AtomicBool::new(false));

    // Start health endpoint
    {
        let health_port = flags.health_port;
        let shutdown_health = Arc::clone(&shutdown);
        thread::spawn(move || start_health_server(health_port, shutdown_health));
    }

    // Start service port listeners (capacity 100 matching Go `make(chan net.Conn, 100)`)
    let (inbound_tx, inbound_rx) = mpsc::sync_channel::<TcpStream>(100);
    for p in &ports {
        let port = p.port;
        let tx = inbound_tx.clone();
        let shutdown_port = Arc::clone(&shutdown);
        thread::spawn(move || listen_service_port(port, tx, shutdown_port));
    }

    // Start Yamux control listener
    let control_addr = format!("0.0.0.0:{}", flags.control_port);
    let control_ln = match TcpListener::bind(&control_addr) {
        Ok(ln) => ln,
        Err(e) => {
            log_info(&format!(
                "failed to listen on control port {}: {e}",
                flags.control_port
            ));
            return 1;
        }
    };
    log_info(&format!(
        "listening for devx CLI on :{}",
        flags.control_port
    ));

    for conn in control_ln.incoming() {
        match conn {
            Ok(stream) => {
                let peer = stream
                    .peer_addr()
                    .map(|a| a.to_string())
                    .unwrap_or_else(|_| "unknown".to_string());
                log_info(&format!("CLI connected from {peer}"));
                handle_cli_session(stream, &inbound_rx, &shutdown);
                log_info("CLI disconnected — triggering self-healing");
                restore_selector(&original_selector, &target_service, &target_namespace);
                shutdown.store(true, Ordering::SeqCst);
                return 0;
            }
            Err(e) => {
                if shutdown.load(Ordering::SeqCst) {
                    return 0;
                }
                log_info(&format!("control accept error: {e}"));
                continue;
            }
        }
    }

    0
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Cursor, Read, Write};

    struct MockRunner {
        calls: Mutex<Vec<(String, Vec<String>)>>,
        should_fail: bool,
    }

    impl MockRunner {
        fn new(should_fail: bool) -> Self {
            Self {
                calls: Mutex::new(Vec::new()),
                should_fail,
            }
        }
    }

    impl CommandRunner for MockRunner {
        fn run_combined(&self, program: &str, args: &[&str]) -> Result<String, String> {
            self.calls.lock().unwrap().push((
                program.to_string(),
                args.iter().map(|s| s.to_string()).collect(),
            ));
            if self.should_fail {
                Err("Forbidden:exit status 1".to_string())
            } else {
                Ok("service/my-svc patched\n".to_string())
            }
        }
    }

    #[test]
    fn test_parse_ports_empty_and_valid() {
        assert!(parse_ports("").is_empty());
        assert_eq!(
            parse_ports("8080:http-api,9090:metrics"),
            vec![
                PortSpec {
                    port: 8080,
                    name: "http-api".to_string()
                },
                PortSpec {
                    port: 9090,
                    name: "metrics".to_string()
                },
            ]
        );
    }

    #[test]
    fn test_parse_ports_optional_name_and_colon_in_name() {
        assert_eq!(
            parse_ports("8080,9090:metrics:v2,badport:skip"),
            vec![
                PortSpec {
                    port: 8080,
                    name: "".to_string()
                },
                PortSpec {
                    port: 9090,
                    name: "metrics:v2".to_string()
                },
            ]
        );
    }

    #[test]
    fn test_parse_flags_defaults_and_overrides() {
        let args = vec!["bridge-agent".to_string()];
        assert_eq!(parse_flags(&args).unwrap(), AgentFlags::default());

        let args2 = vec![
            "bridge-agent".to_string(),
            "--control-port=5000".to_string(),
            "-health-port".to_string(),
            "5001".to_string(),
            "--ports=8080:http,9090:grpc".to_string(),
        ];
        assert_eq!(
            parse_flags(&args2).unwrap(),
            AgentFlags {
                control_port: 5000,
                health_port: 5001,
                ports: "8080:http,9090:grpc".to_string(),
            }
        );
    }

    #[test]
    fn test_parse_flags_errors_on_unknown_or_bad_int() {
        let bad_flag = vec!["bridge-agent".to_string(), "--unknown=1".to_string()];
        assert!(parse_flags(&bad_flag).is_err());

        let bad_int = vec![
            "bridge-agent".to_string(),
            "--control-port=notanint".to_string(),
        ];
        assert!(parse_flags(&bad_int).is_err());
    }

    #[test]
    fn test_build_restore_selector_patch_sorts_keys_deterministically() {
        let raw = r#"{"version":"v1","app":"checkout","tier":"backend"}"#;
        let patch = build_restore_selector_patch(raw).unwrap();
        assert_eq!(
            patch,
            r#"{"metadata":{"annotations":{"devx-bridge-session":null}},"spec":{"selector":{"app":"checkout","tier":"backend","version":"v1"}}}"#
        );
    }

    #[test]
    fn test_restore_selector_skips_missing_env_and_invokes_kubectl() {
        let runner = MockRunner::new(false);
        assert!(restore_selector_with_runner("", "svc", "ns", &runner).is_ok());
        assert!(runner.calls.lock().unwrap().is_empty());

        assert!(
            restore_selector_with_runner(r#"{"app":"web"}"#, "web-svc", "prod", &runner).is_ok()
        );
        let calls = runner.calls.lock().unwrap();
        assert_eq!(calls.len(), 1);
        assert_eq!(calls[0].0, "kubectl");
        assert_eq!(
            calls[0].1,
            vec![
                "patch",
                "service",
                "web-svc",
                "-n",
                "prod",
                "--type=merge",
                "-p",
                r#"{"metadata":{"annotations":{"devx-bridge-session":null}},"spec":{"selector":{"app":"web"}}}"#,
            ]
        );
    }

    #[test]
    fn test_restore_selector_handles_invalid_json_and_command_failure() {
        let runner = MockRunner::new(false);
        assert!(restore_selector_with_runner("not-json", "svc", "ns", &runner).is_err());
        assert!(runner.calls.lock().unwrap().is_empty());

        let failing_runner = MockRunner::new(true);
        assert!(
            restore_selector_with_runner(r#"{"app":"web"}"#, "svc", "ns", &failing_runner).is_err()
        );
        assert_eq!(failing_runner.calls.lock().unwrap().len(), 1);
    }

    #[test]
    fn test_health_routes_and_http_response() {
        assert_eq!(
            route_health_request("GET /healthz HTTP/1.1\r\n"),
            (200, "ok", false)
        );
        assert_eq!(
            route_health_request("POST /healthz?probe=1 HTTP/1.1\r\n"),
            (200, "ok", false)
        );
        assert_eq!(
            route_health_request("HEAD /healthz HTTP/1.1\r\n"),
            (200, "ok", true)
        );
        assert_eq!(
            route_health_request("GET /other HTTP/1.1\r\n"),
            (404, "404 page not found\n", false)
        );

        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let addr = listener.local_addr().unwrap();
        let shutdown = Arc::new(AtomicBool::new(false));
        let shutdown_bg = Arc::clone(&shutdown);
        let server_thread = thread::spawn(move || {
            if let Ok((conn, _)) = listener.accept() {
                let _ = handle_health_conn(conn);
            }
            shutdown_bg.store(true, Ordering::SeqCst);
        });

        let mut client = TcpStream::connect(addr).unwrap();
        client
            .write_all(b"GET /healthz HTTP/1.1\r\nHost: localhost\r\n\r\n")
            .unwrap();
        let mut resp = String::new();
        client.read_to_string(&mut resp).unwrap();
        assert!(resp.starts_with("HTTP/1.1 200 OK\r\n"));
        assert!(resp.ends_with("\r\n\r\nok"));
        server_thread.join().unwrap();
    }

    #[test]
    fn test_yamux_header_roundtrip_and_session_proxy() {
        let hdr = YamuxHeader {
            version: YAMUX_VERSION,
            msg_type: YAMUX_TYPE_DATA,
            flags: YAMUX_FLAG_SYN | YAMUX_FLAG_ACK,
            stream_id: 2,
            length: 1024,
        };
        let encoded = hdr.encode();
        assert_eq!(YamuxHeader::decode(&encoded), hdr);

        // Verify full Yamux server session + proxy_to_stream roundtrip over localhost TCP
        let control_ln = TcpListener::bind("127.0.0.1:0").unwrap();
        let control_addr = control_ln.local_addr().unwrap();

        let service_ln = TcpListener::bind("127.0.0.1:0").unwrap();
        let service_addr = service_ln.local_addr().unwrap();

        let shutdown = Arc::new(AtomicBool::new(false));
        let (inbound_tx, inbound_rx) = mpsc::sync_channel::<TcpStream>(10);

        let shutdown_svc = Arc::clone(&shutdown);
        thread::spawn(move || {
            listen_service_port_on(
                service_ln,
                service_addr.port() as i32,
                inbound_tx,
                shutdown_svc,
            );
        });

        let shutdown_cli = Arc::clone(&shutdown);
        let agent_thread = thread::spawn(move || {
            let (cli_conn, _) = control_ln.accept().unwrap();
            handle_cli_session(cli_conn, &inbound_rx, &shutdown_cli);
        });

        // Connect simulated CLI (Yamux client) to control port
        let mut cli_sock = TcpStream::connect(control_addr).unwrap();

        // Connect simulated cluster client to service port and send payload
        let client_thread = thread::spawn(move || {
            let mut svc_client = TcpStream::connect(service_addr).unwrap();
            svc_client.write_all(b"ping-from-cluster").unwrap();
            let mut reply = [0u8; 13];
            svc_client.read_exact(&mut reply).unwrap();
            assert_eq!(&reply, b"pong-from-cli");
        });

        // CLI reads SYN WindowUpdate frame for stream 2, then Data frame
        let mut hdr_buf = [0u8; YAMUX_HEADER_LEN];
        cli_sock.read_exact(&mut hdr_buf).unwrap();
        let syn_hdr = YamuxHeader::decode(&hdr_buf);
        assert_eq!(syn_hdr.msg_type, YAMUX_TYPE_WINDOW_UPDATE);
        assert_eq!(syn_hdr.flags, YAMUX_FLAG_SYN);
        assert_eq!(syn_hdr.stream_id, 2);

        cli_sock.read_exact(&mut hdr_buf).unwrap();
        let data_hdr = YamuxHeader::decode(&hdr_buf);
        assert_eq!(data_hdr.msg_type, YAMUX_TYPE_DATA);
        assert_eq!(data_hdr.stream_id, 2);
        let mut payload = vec![0u8; data_hdr.length as usize];
        cli_sock.read_exact(&mut payload).unwrap();
        assert_eq!(&payload, b"ping-from-cluster");

        // CLI sends response Data frame + FIN on stream 2, then closes socket
        let resp_payload = b"pong-from-cli";
        let resp_hdr = YamuxHeader {
            version: YAMUX_VERSION,
            msg_type: YAMUX_TYPE_DATA,
            flags: YAMUX_FLAG_FIN,
            stream_id: 2,
            length: resp_payload.len() as u32,
        };
        cli_sock.write_all(&resp_hdr.encode()).unwrap();
        cli_sock.write_all(resp_payload).unwrap();
        cli_sock.flush().unwrap();

        client_thread.join().unwrap();
        drop(cli_sock);
        agent_thread.join().unwrap();
    }

    #[test]
    fn test_handle_health_conn_head_omits_body() {
        struct ReadWriteBuf {
            input: Cursor<Vec<u8>>,
            output: Vec<u8>,
        }
        impl Read for ReadWriteBuf {
            fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
                self.input.read(buf)
            }
        }
        impl Write for ReadWriteBuf {
            fn write(&mut self, buf: &[u8]) -> io::Result<usize> {
                self.output.extend_from_slice(buf);
                Ok(buf.len())
            }
            fn flush(&mut self) -> io::Result<()> {
                Ok(())
            }
        }

        let mut rw = ReadWriteBuf {
            input: Cursor::new(b"HEAD /healthz HTTP/1.1\r\n\r\n".to_vec()),
            output: Vec::new(),
        };
        handle_health_conn(&mut rw).unwrap();
        let out_str = String::from_utf8(rw.output).unwrap();
        assert!(out_str.starts_with("HTTP/1.1 200 OK\r\n"));
        assert!(out_str.contains("Content-Length: 2\r\n"));
        assert!(out_str.ends_with("\r\n\r\n"));
    }
}
