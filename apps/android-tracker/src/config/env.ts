// Android emulator: 10.0.2.2 routes to the host machine's localhost.
// Real device on LAN: replace with your machine's LAN IP, e.g. 192.168.1.42.
const HOST = '192.168.31.78';
const PORT = '8080';

export const API_BASE_URL = `http://${HOST}:${PORT}`;
export const WS_URL = `ws://${HOST}:${PORT}/v1/ws/tracker`;
