"""Local-only TCP relay from Docker Desktop to a Tailscale Proxmox API.

Run on the Windows host when Docker Desktop cannot route directly to the
Tailscale interface. It forwards opaque TLS bytes and accepts only Docker
Desktop's private subnet or loopback; it never stores credentials.
"""

import argparse
import asyncio
import contextlib
import ipaddress


async def copy(reader: asyncio.StreamReader, writer: asyncio.StreamWriter) -> None:
    try:
        while data := await reader.read(64 * 1024):
            writer.write(data)
            await writer.drain()
    finally:
        writer.close()
        with contextlib.suppress(Exception):
            await writer.wait_closed()


async def relay(client_reader: asyncio.StreamReader, client_writer: asyncio.StreamWriter, host: str, port: int) -> None:
    peer = client_writer.get_extra_info("peername")
    try:
        address = ipaddress.ip_address(peer[0])
        if not (address.is_loopback or address in ipaddress.ip_network("192.168.65.0/24")):
            client_writer.close()
            return
    except (TypeError, ValueError):
        client_writer.close()
        return
    try:
        remote_reader, remote_writer = await asyncio.wait_for(asyncio.open_connection(host, port), timeout=10)
    except Exception:
        client_writer.close()
        with contextlib.suppress(Exception):
            await client_writer.wait_closed()
        return
    await asyncio.gather(copy(client_reader, remote_writer), copy(remote_reader, client_writer))


async def main() -> None:
    parser = argparse.ArgumentParser()
    # Docker Desktop reaches the Windows host through 192.168.65.0/24. The
    # relay accepts only that subnet and loopback, even when it binds all IPv4.
    parser.add_argument("--listen-host", default="0.0.0.0")
    parser.add_argument("--listen-port", type=int, default=8006)
    parser.add_argument("--target-host", required=True)
    parser.add_argument("--target-port", type=int, default=8006)
    args = parser.parse_args()
    server = await asyncio.start_server(
        lambda reader, writer: relay(reader, writer, args.target_host, args.target_port),
        args.listen_host,
        args.listen_port,
    )
    async with server:
        await server.serve_forever()


if __name__ == "__main__":
    asyncio.run(main())
