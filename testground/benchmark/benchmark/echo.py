import asyncio
import threading


async def handle_echo(reader: asyncio.StreamReader, writer: asyncio.StreamWriter):
    data = await reader.read(1024)
    writer.write(data)
    await writer.drain()

    print("Close the connection")
    writer.close()
    await writer.wait_closed()


async def echo_server(port: int):
    server = await asyncio.start_server(handle_echo, "0.0.0.0", port)
    async with server:
        await server.serve_forever()


def run_echo_server(port: int):
    """
    run async echo server in a thread.
    """
    threading.Thread(target=asyncio.run, args=(echo_server(port),)).start()
