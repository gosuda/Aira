export type WSHandler = (event: string, data: unknown) => void;

export function createWSConnection(
	path: string,
	token: string,
	handler: WSHandler
): { close: () => void } {
	let ws: WebSocket | null = null;
	let closed = false;
	let retryDelay = 1000;

	function connect() {
		if (closed) return;

		const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
		const url = `${protocol}//${window.location.host}${path}?token=${encodeURIComponent(token)}`;

		ws = new WebSocket(url);

		ws.onopen = () => {
			retryDelay = 1000;
		};

		ws.onmessage = (ev) => {
			try {
				const msg = JSON.parse(ev.data as string);
				handler(msg.event ?? 'message', msg.data ?? msg);
			} catch {
				// Ignore non-JSON messages
			}
		};

		ws.onclose = () => {
			if (!closed) {
				setTimeout(connect, retryDelay);
				retryDelay = Math.min(retryDelay * 2, 30000);
			}
		};

		ws.onerror = () => {
			ws?.close();
		};
	}

	connect();

	return {
		close() {
			closed = true;
			ws?.close();
		}
	};
}
