use std::collections::HashMap;
use std::env;
use std::error::Error;
use std::fs;
use std::fs::File;
use std::io::{self, Read, Write};
use std::os::fd::AsFd;
use std::os::fd::OwnedFd;
use std::os::unix::net::{UnixListener, UnixStream};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use wayland_protocols::ext::data_control::v1::server::ext_data_control_device_v1::{
    self, ExtDataControlDeviceV1,
};
use wayland_protocols::ext::data_control::v1::server::ext_data_control_manager_v1::{
    self, ExtDataControlManagerV1,
};
use wayland_protocols::ext::data_control::v1::server::ext_data_control_offer_v1::ExtDataControlOfferV1;
use wayland_protocols::ext::data_control::v1::server::ext_data_control_source_v1::{
    self, ExtDataControlSourceV1,
};
use wayland_server::backend::{ClientData, ClientId, DisconnectReason, ObjectId};
use wayland_server::protocol::wl_data_device::{self, WlDataDevice};
use wayland_server::protocol::wl_data_device_manager::{self, WlDataDeviceManager};
use wayland_server::protocol::wl_data_offer::{self, WlDataOffer};
use wayland_server::protocol::wl_data_source::{self, WlDataSource};
use wayland_server::protocol::wl_seat::{self, WlSeat};
use wayland_server::{
    Client, DataInit, Dispatch, Display, DisplayHandle, GlobalDispatch, ListeningSocket, New,
    Resource,
};

const DEFAULT_SOCKET: &str = "arc-clipd-0";
const DEFAULT_SEAT: &str = "seat0";

#[derive(Debug)]
struct PeerData;

impl ClientData for PeerData {
    fn initialized(&self, _client_id: ClientId) {}
    fn disconnected(&self, _client_id: ClientId, _reason: DisconnectReason) {}
}

#[derive(Debug, Clone)]
struct SeatData;

#[derive(Debug, Clone)]
struct SelectionDeviceData;

#[derive(Debug, Clone)]
struct OfferData {
    mime_types: Vec<String>,
    source: ClipboardSource,
}

#[derive(Debug, Clone)]
struct StoredSelection {
    mime_types: Vec<String>,
    bytes: Arc<Vec<u8>>,
}

#[derive(Debug, Clone)]
enum ClipboardSource {
    Core(WlDataSource),
    Ext(ExtDataControlSourceV1),
    Internal(Arc<StoredSelection>),
}

impl ClipboardSource {
    fn id(&self) -> Option<ObjectId> {
        match self {
            ClipboardSource::Core(source) => Some(source.id()),
            ClipboardSource::Ext(source) => Some(source.id()),
            ClipboardSource::Internal(_) => None,
        }
    }

    fn is_alive(&self) -> bool {
        match self {
            ClipboardSource::Core(source) => source.is_alive(),
            ClipboardSource::Ext(source) => source.is_alive(),
            ClipboardSource::Internal(_) => true,
        }
    }

    fn mime_types(&self, state: &State) -> Vec<String> {
        match self {
            ClipboardSource::Core(source) => state
                .core_sources
                .get(&source.id())
                .cloned()
                .unwrap_or_default(),
            ClipboardSource::Ext(source) => state
                .ext_sources
                .get(&source.id())
                .cloned()
                .unwrap_or_default(),
            ClipboardSource::Internal(selection) => selection.mime_types.clone(),
        }
    }

    fn send(&self, mime_type: String, fd: OwnedFd) {
        match self {
            ClipboardSource::Core(source) => {
                let _ = source.send(mime_type, fd.as_fd());
            }
            ClipboardSource::Ext(source) => {
                let _ = source.send(mime_type, fd.as_fd());
            }
            ClipboardSource::Internal(selection) => {
                if selection
                    .mime_types
                    .iter()
                    .any(|candidate| candidate == &mime_type)
                {
                    let mut file = File::from(fd);
                    let _ = file.write_all(selection.bytes.as_slice());
                    let _ = file.flush();
                }
            }
        }
    }
}

#[derive(Debug, Clone)]
struct SelectionState {
    mime_types: Vec<String>,
    source: ClipboardSource,
}

#[derive(Debug)]
struct State {
    seat_name: String,
    seats: Vec<WlSeat>,
    core_devices: Vec<WlDataDevice>,
    ext_devices: Vec<ExtDataControlDeviceV1>,
    core_sources: HashMap<ObjectId, Vec<String>>,
    ext_sources: HashMap<ObjectId, Vec<String>>,
    selection: Option<SelectionState>,
}

impl State {
    fn new(seat_name: String) -> Self {
        Self {
            seat_name,
            seats: Vec::new(),
            core_devices: Vec::new(),
            ext_devices: Vec::new(),
            core_sources: HashMap::new(),
            ext_sources: HashMap::new(),
            selection: None,
        }
    }

    fn set_selection(&mut self, source: ClipboardSource) {
        let mime_types = source.mime_types(self);
        if mime_types.is_empty() {
            self.selection = None;
            return;
        }
        self.selection = Some(SelectionState { mime_types, source });
    }

    fn set_internal_selection(&mut self, mime_type: String, bytes: Vec<u8>) {
        let selection = StoredSelection {
            mime_types: vec![mime_type],
            bytes: Arc::new(bytes),
        };
        self.selection = Some(SelectionState {
            mime_types: selection.mime_types.clone(),
            source: ClipboardSource::Internal(Arc::new(selection)),
        });
    }

    fn cleanup(&mut self) {
        self.seats.retain(Resource::is_alive);
        self.core_devices.retain(Resource::is_alive);
        self.ext_devices.retain(Resource::is_alive);
        self.core_sources.retain(|id, _| {
            self.selection.as_ref().is_none_or(|selection| {
                selection.source.id() != Some(id.clone()) || selection.source.is_alive()
            })
        });
        self.ext_sources.retain(|id, _| {
            self.selection.as_ref().is_none_or(|selection| {
                selection.source.id() != Some(id.clone()) || selection.source.is_alive()
            })
        });
        if self
            .selection
            .as_ref()
            .is_some_and(|selection| !selection.source.is_alive())
        {
            self.selection = None;
        }
    }

    fn broadcast_selection(&mut self, dh: &DisplayHandle) {
        self.cleanup();
        let selection = self.selection.clone();
        for device in self.core_devices.clone() {
            self.send_core_selection(dh, &device, selection.clone());
        }
        for device in self.ext_devices.clone() {
            self.send_ext_selection(dh, &device, selection.clone());
        }
    }

    fn send_core_selection(
        &self,
        dh: &DisplayHandle,
        device: &WlDataDevice,
        selection: Option<SelectionState>,
    ) {
        let Some(client) = device.client() else {
            return;
        };
        let Some(selection) = selection else {
            let _ = device.selection(None);
            return;
        };

        let Ok(offer) = client.create_resource::<WlDataOffer, _, State>(
            dh,
            3,
            OfferData {
                mime_types: selection.mime_types.clone(),
                source: selection.source.clone(),
            },
        ) else {
            return;
        };

        let _ = device.data_offer(&offer);
        for mime in &selection.mime_types {
            let _ = offer.offer(mime.clone());
        }
        let _ = device.selection(Some(&offer));
    }

    fn send_ext_selection(
        &self,
        dh: &DisplayHandle,
        device: &ExtDataControlDeviceV1,
        selection: Option<SelectionState>,
    ) {
        let Some(client) = device.client() else {
            return;
        };
        let Some(selection) = selection else {
            let _ = device.selection(None);
            return;
        };

        let Ok(offer) = client.create_resource::<ExtDataControlOfferV1, _, State>(
            dh,
            1,
            OfferData {
                mime_types: selection.mime_types.clone(),
                source: selection.source.clone(),
            },
        ) else {
            return;
        };

        let _ = device.data_offer(&offer);
        for mime in &selection.mime_types {
            let _ = offer.offer(mime.clone());
        }
        let _ = device.selection(Some(&offer));
    }
}

fn parse_arg(flag: &str, default: &str) -> String {
    let mut args = env::args().skip(1);
    while let Some(arg) = args.next() {
        if arg == flag {
            if let Some(value) = args.next() {
                return value;
            }
        }
    }
    default.to_string()
}

fn runtime_dir() -> Result<String, Box<dyn Error>> {
    match env::var("XDG_RUNTIME_DIR") {
        Ok(dir) if !dir.trim().is_empty() => Ok(dir),
        _ => Err("XDG_RUNTIME_DIR is not set".into()),
    }
}

fn control_socket_path(socket_name: &str) -> Result<String, Box<dyn Error>> {
    Ok(format!("{}/{}.control", runtime_dir()?, socket_name))
}

fn insert_client(dh: &DisplayHandle, stream: UnixStream) -> Result<(), Box<dyn Error>> {
    let mut dh = dh.clone();
    let _ = dh.insert_client(stream, Arc::new(PeerData))?;
    Ok(())
}

fn write_control_message(
    mut stream: UnixStream,
    mime_type: &str,
    bytes: &[u8],
) -> Result<(), Box<dyn Error>> {
    let mime = mime_type.as_bytes();
    stream.write_all(&(mime.len() as u32).to_be_bytes())?;
    stream.write_all(&(bytes.len() as u64).to_be_bytes())?;
    stream.write_all(mime)?;
    stream.write_all(bytes)?;
    stream.flush()?;
    Ok(())
}

fn read_control_message(mut stream: UnixStream) -> Result<(String, Vec<u8>), Box<dyn Error>> {
    let mut mime_len_buf = [0u8; 4];
    stream.read_exact(&mut mime_len_buf)?;
    let mime_len = u32::from_be_bytes(mime_len_buf) as usize;

    let mut data_len_buf = [0u8; 8];
    stream.read_exact(&mut data_len_buf)?;
    let data_len = u64::from_be_bytes(data_len_buf) as usize;

    let mut mime_buf = vec![0u8; mime_len];
    stream.read_exact(&mut mime_buf)?;
    let mime_type = String::from_utf8(mime_buf)?;

    let mut data = vec![0u8; data_len];
    stream.read_exact(&mut data)?;
    Ok((mime_type, data))
}

fn run_put(socket_name: &str, mime_type: &str) -> Result<(), Box<dyn Error>> {
    let mut bytes = Vec::new();
    io::stdin().read_to_end(&mut bytes)?;
    let path = control_socket_path(socket_name)?;
    let stream = UnixStream::connect(path)?;
    write_control_message(stream, mime_type, &bytes)
}

fn run_server(socket_name: String, seat_name: String) -> Result<(), Box<dyn Error>> {
    let mut display = Display::<State>::new()?;
    let dh = display.handle();
    let socket = ListeningSocket::bind(&socket_name)?;

    let control_path = control_socket_path(&socket_name)?;
    let _ = fs::remove_file(&control_path);
    let control_listener = UnixListener::bind(&control_path)?;
    control_listener.set_nonblocking(true)?;

    let mut state = State::new(seat_name);

    dh.create_global::<State, WlSeat, _>(7, ());
    dh.create_global::<State, WlDataDeviceManager, _>(3, ());
    dh.create_global::<State, ExtDataControlManagerV1, _>(1, ());

    loop {
        while let Some(stream) = socket.accept()? {
            insert_client(&dh, stream)?;
        }

        loop {
            match control_listener.accept() {
                Ok((stream, _)) => {
                    let (mime_type, data) = read_control_message(stream)?;
                    state.set_internal_selection(mime_type, data);
                    state.broadcast_selection(&dh);
                }
                Err(err) if err.kind() == io::ErrorKind::WouldBlock => break,
                Err(err) => return Err(Box::new(err)),
            }
        }

        let _ = display.dispatch_clients(&mut state)?;
        display.flush_clients()?;
        thread::sleep(Duration::from_millis(10));
    }
}

fn main() -> Result<(), Box<dyn Error>> {
    let args: Vec<String> = env::args().collect();
    if args.get(1).is_some_and(|arg| arg == "put") {
        let socket_name = parse_arg("--socket", DEFAULT_SOCKET);
        let mime_type = parse_arg("--type", "");
        if mime_type.is_empty() {
            return Err("missing --type for clipd put".into());
        }
        return run_put(&socket_name, &mime_type);
    }

    let socket_name = parse_arg("--socket", DEFAULT_SOCKET);
    let seat_name = parse_arg("--seat", DEFAULT_SEAT);
    run_server(socket_name, seat_name)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;

    static ENV_LOCK: Mutex<()> = Mutex::new(());

    #[test]
    fn parse_arg_returns_default_when_flag_missing() {
        assert_eq!(parse_arg("--missing", "fallback"), "fallback");
    }

    #[test]
    fn write_and_read_control_message_round_trip() {
        let (writer, reader) = UnixStream::pair().expect("create unix stream pair");
        write_control_message(writer, "image/png", b"payload").expect("write control message");
        let (mime, bytes) = read_control_message(reader).expect("read control message");
        assert_eq!(mime, "image/png");
        assert_eq!(bytes, b"payload");
    }

    #[test]
    fn control_socket_path_uses_runtime_dir() {
        let _guard = ENV_LOCK.lock().expect("env lock");
        let temp = env::temp_dir().join(format!("arc-clipd-test-{}", std::process::id()));
        fs::create_dir_all(&temp).expect("create runtime dir");
        let prev = env::var("XDG_RUNTIME_DIR").ok();
        env::set_var("XDG_RUNTIME_DIR", &temp);

        let path = control_socket_path("arc-clipd-0").expect("control socket path");
        assert_eq!(path, format!("{}/arc-clipd-0.control", temp.display()));

        match prev {
            Some(value) => env::set_var("XDG_RUNTIME_DIR", value),
            None => env::remove_var("XDG_RUNTIME_DIR"),
        }
        let _ = fs::remove_dir_all(&temp);
    }
}

impl GlobalDispatch<WlSeat, (), State> for State {
    fn bind(
        state: &mut State,
        _dh: &DisplayHandle,
        _client: &Client,
        resource: New<WlSeat>,
        _global_data: &(),
        data_init: &mut DataInit<'_, State>,
    ) {
        let seat = data_init.init(resource, SeatData);
        let _ = seat.name(state.seat_name.clone());
        let _ = seat.capabilities(wl_seat::Capability::empty());
        state.seats.push(seat);
    }
}

impl Dispatch<WlSeat, SeatData, State> for State {
    fn request(
        _state: &mut State,
        _client: &Client,
        seat: &WlSeat,
        request: wl_seat::Request,
        _data: &SeatData,
        _dh: &DisplayHandle,
        _data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            wl_seat::Request::Release => {}
            wl_seat::Request::GetPointer { .. }
            | wl_seat::Request::GetKeyboard { .. }
            | wl_seat::Request::GetTouch { .. } => {
                seat.post_error(
                    wl_seat::Error::MissingCapability,
                    "arc-clipd does not implement pointer, keyboard, or touch devices",
                );
            }
            _ => {}
        }
    }
}

impl GlobalDispatch<WlDataDeviceManager, (), State> for State {
    fn bind(
        _state: &mut State,
        _dh: &DisplayHandle,
        _client: &Client,
        resource: New<WlDataDeviceManager>,
        _global_data: &(),
        data_init: &mut DataInit<'_, State>,
    ) {
        let _ = data_init.init(resource, ());
    }
}

impl Dispatch<WlDataDeviceManager, (), State> for State {
    fn request(
        state: &mut State,
        _client: &Client,
        _resource: &WlDataDeviceManager,
        request: wl_data_device_manager::Request,
        _data: &(),
        dh: &DisplayHandle,
        data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            wl_data_device_manager::Request::CreateDataSource { id } => {
                let _ = data_init.init(id, Vec::<String>::new());
            }
            wl_data_device_manager::Request::GetDataDevice { id, .. } => {
                let device = data_init.init(id, SelectionDeviceData);
                state.core_devices.push(device.clone());
                state.send_core_selection(dh, &device, state.selection.clone());
            }
            _ => {}
        }
    }
}

impl Dispatch<WlDataSource, Vec<String>, State> for State {
    fn request(
        state: &mut State,
        _client: &Client,
        source: &WlDataSource,
        request: wl_data_source::Request,
        mime_types: &Vec<String>,
        _dh: &DisplayHandle,
        _data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            wl_data_source::Request::Offer { mime_type } => {
                let mut next = mime_types.clone();
                next.push(mime_type);
                state.core_sources.insert(source.id(), next);
            }
            wl_data_source::Request::Destroy => {}
            wl_data_source::Request::SetActions { .. } => {}
            _ => {}
        }
    }

    fn destroyed(state: &mut State, _client: ClientId, source: &WlDataSource, _data: &Vec<String>) {
        state.core_sources.remove(&source.id());
        if state
            .selection
            .as_ref()
            .is_some_and(|selection| selection.source.id() == Some(source.id()))
        {
            state.selection = None;
        }
    }
}

impl Dispatch<WlDataDevice, SelectionDeviceData, State> for State {
    fn request(
        state: &mut State,
        _client: &Client,
        _device: &WlDataDevice,
        request: wl_data_device::Request,
        _data: &SelectionDeviceData,
        dh: &DisplayHandle,
        _data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            wl_data_device::Request::SetSelection { source, .. } => {
                if let Some(source) = source {
                    state.set_selection(ClipboardSource::Core(source));
                } else {
                    state.selection = None;
                }
                state.broadcast_selection(dh);
            }
            wl_data_device::Request::Release => {}
            _ => {}
        }
    }
}

impl Dispatch<WlDataOffer, OfferData, State> for State {
    fn request(
        _state: &mut State,
        _client: &Client,
        _offer: &WlDataOffer,
        request: wl_data_offer::Request,
        data: &OfferData,
        _dh: &DisplayHandle,
        _data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            wl_data_offer::Request::Receive { mime_type, fd } => {
                if data
                    .mime_types
                    .iter()
                    .any(|candidate| candidate == &mime_type)
                {
                    data.source.send(mime_type, fd);
                }
            }
            wl_data_offer::Request::Accept { .. }
            | wl_data_offer::Request::Destroy
            | wl_data_offer::Request::Finish
            | wl_data_offer::Request::SetActions { .. } => {}
            _ => {}
        }
    }
}

impl GlobalDispatch<ExtDataControlManagerV1, (), State> for State {
    fn bind(
        _state: &mut State,
        _dh: &DisplayHandle,
        _client: &Client,
        resource: New<ExtDataControlManagerV1>,
        _global_data: &(),
        data_init: &mut DataInit<'_, State>,
    ) {
        let _ = data_init.init(resource, ());
    }
}

impl Dispatch<ExtDataControlManagerV1, (), State> for State {
    fn request(
        state: &mut State,
        _client: &Client,
        _resource: &ExtDataControlManagerV1,
        request: ext_data_control_manager_v1::Request,
        _data: &(),
        dh: &DisplayHandle,
        data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            ext_data_control_manager_v1::Request::CreateDataSource { id } => {
                let _ = data_init.init(id, Vec::<String>::new());
            }
            ext_data_control_manager_v1::Request::GetDataDevice { id, .. } => {
                let device = data_init.init(id, SelectionDeviceData);
                state.ext_devices.push(device.clone());
                state.send_ext_selection(dh, &device, state.selection.clone());
            }
            ext_data_control_manager_v1::Request::Destroy => {}
            _ => {}
        }
    }
}

impl Dispatch<ExtDataControlSourceV1, Vec<String>, State> for State {
    fn request(
        state: &mut State,
        _client: &Client,
        source: &ExtDataControlSourceV1,
        request: ext_data_control_source_v1::Request,
        mime_types: &Vec<String>,
        _dh: &DisplayHandle,
        _data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            ext_data_control_source_v1::Request::Offer { mime_type } => {
                let mut next = mime_types.clone();
                next.push(mime_type);
                state.ext_sources.insert(source.id(), next);
            }
            ext_data_control_source_v1::Request::Destroy => {}
            _ => {}
        }
    }

    fn destroyed(
        state: &mut State,
        _client: ClientId,
        source: &ExtDataControlSourceV1,
        _data: &Vec<String>,
    ) {
        state.ext_sources.remove(&source.id());
        if state
            .selection
            .as_ref()
            .is_some_and(|selection| selection.source.id() == Some(source.id()))
        {
            state.selection = None;
        }
    }
}

impl Dispatch<ExtDataControlDeviceV1, SelectionDeviceData, State> for State {
    fn request(
        state: &mut State,
        _client: &Client,
        _device: &ExtDataControlDeviceV1,
        request: ext_data_control_device_v1::Request,
        _data: &SelectionDeviceData,
        dh: &DisplayHandle,
        _data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            ext_data_control_device_v1::Request::SetSelection { source } => {
                if let Some(source) = source {
                    state.set_selection(ClipboardSource::Ext(source));
                } else {
                    state.selection = None;
                }
                state.broadcast_selection(dh);
            }
            ext_data_control_device_v1::Request::Destroy => {}
            _ => {}
        }
    }
}

impl Dispatch<ExtDataControlOfferV1, OfferData, State> for State {
    fn request(
        _state: &mut State,
        _client: &Client,
        _offer: &ExtDataControlOfferV1,
        request: wayland_protocols::ext::data_control::v1::server::ext_data_control_offer_v1::Request,
        data: &OfferData,
        _dh: &DisplayHandle,
        _data_init: &mut DataInit<'_, State>,
    ) {
        match request {
            wayland_protocols::ext::data_control::v1::server::ext_data_control_offer_v1::Request::Receive {
                mime_type,
                fd,
            } => {
                if data.mime_types.iter().any(|candidate| candidate == &mime_type) {
                    data.source.send(mime_type, fd);
                }
            }
            wayland_protocols::ext::data_control::v1::server::ext_data_control_offer_v1::Request::Destroy => {}
            _ => {}
        }
    }
}
