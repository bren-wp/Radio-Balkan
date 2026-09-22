#!/usr/bin/env python3
"""Deterministic regression tests for the live-radio CI network boundary."""

from __future__ import annotations

import importlib.util
import pathlib
import socket

ROOT = pathlib.Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("live_radio_probe", ROOT / "test_live_radio_streams.py")
if SPEC is None or SPEC.loader is None:
    raise RuntimeError("cannot load live radio probe module")
live = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(live)


class FakeSocket:
    pass


class FakeTLSContext:
    def __init__(self):
        self.calls = []
        self.verify_mode = 2
        self.check_hostname = True
        self.post_handshake_auth = False

    def wrap_socket(self, sock, *, server_hostname=None):
        self.calls.append((sock, server_hostname))
        return sock


def test_http_connection_uses_pinned_address():
    calls = []
    original = socket.create_connection
    try:
        socket.create_connection = lambda address, timeout=None, source_address=None: (
            calls.append((address, timeout, source_address)) or FakeSocket()
        )
        conn = live.PinnedHTTPConnection(
            "radio.example:8080",
            pinned_addresses=["93.184.216.34"],
            timeout=3,
        )
        conn.connect()
    finally:
        socket.create_connection = original

    assert calls, "HTTP pinned connection did not create a socket"
    assert calls[0][0] == ("93.184.216.34", 8080), calls[0]


def test_https_connection_uses_pinned_address_and_original_sni():
    calls = []
    original = socket.create_connection
    context = FakeTLSContext()
    try:
        socket.create_connection = lambda address, timeout=None, source_address=None: (
            calls.append((address, timeout, source_address)) or FakeSocket()
        )
        conn = live.PinnedHTTPSConnection(
            "secure-radio.example:443",
            pinned_addresses=["93.184.216.34"],
            timeout=3,
            context=context,
        )
        conn.connect()
    finally:
        socket.create_connection = original

    assert calls, "HTTPS pinned connection did not create a socket"
    assert calls[0][0] == ("93.184.216.34", 443), calls[0]
    assert context.calls, "HTTPS pinned connection did not wrap TLS"
    assert context.calls[0][1] == "secure-radio.example", context.calls[0]


def test_http_connection_falls_back_across_validated_addresses():
    calls = []
    original = socket.create_connection

    def fake_create_connection(address, timeout=None, source_address=None):
        calls.append((address, timeout, source_address))
        if address[0] == "93.184.216.35":
            raise OSError("first validated address unavailable")
        return FakeSocket()

    try:
        socket.create_connection = fake_create_connection
        conn = live.PinnedHTTPConnection(
            "radio.example:8080",
            pinned_addresses=["93.184.216.35", "93.184.216.34"],
            timeout=3,
        )
        conn.connect()
    finally:
        socket.create_connection = original

    assert [entry[0] for entry in calls] == [
        ("93.184.216.35", 8080),
        ("93.184.216.34", 8080),
    ], calls


def test_validation_rejects_non_global_literal():
    try:
        live.validate_public_url("http://127.0.0.1:8000/live")
    except ValueError:
        return
    raise AssertionError("loopback URL passed public-target validation")


def test_validation_rejects_mixed_public_private_dns_answers():
    original = live._public_addresses
    try:
        live._public_addresses = lambda hostname: [
            live.ipaddress.ip_address("93.184.216.34"),
            live.ipaddress.ip_address("127.0.0.1"),
        ]
        try:
            live.validate_public_url("https://radio.example/live")
        except ValueError:
            return
        raise AssertionError("mixed public/private DNS answer passed validation")
    finally:
        live._public_addresses = original


def main():
    tests = (
        test_http_connection_uses_pinned_address,
        test_https_connection_uses_pinned_address_and_original_sni,
        test_http_connection_falls_back_across_validated_addresses,
        test_validation_rejects_non_global_literal,
        test_validation_rejects_mixed_public_private_dns_answers,
    )
    for test in tests:
        test()
        print(f"[live-radio-security] PASS {test.__name__}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
