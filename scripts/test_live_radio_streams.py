#!/usr/bin/env python3
"""Real-network smoke for Balkan radio streams.

This complements the Windows MediaPlayer runtime smoke. GitHub's hosted Windows
runner may not expose an audio endpoint, so this gate verifies that the same
kind of live internet streams the product consumes are actually reachable:
MP3, AAC/AAC+, HTTPS and Radio Browser raw->resolved indirection.

It intentionally discovers current stations instead of hardcoding stream URLs.
A single dead station does not fail the build; several ranked candidates are
tried for every required capability.
"""

from __future__ import annotations

import http.client
import ipaddress
import json
import socket
import ssl
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

COUNTRIES = ("HR", "BA", "RS", "SI", "ME", "MK", "BG", "AL")
FALLBACK_SERVERS = (
    "https://de1.api.radio-browser.info",
    "https://de2.api.radio-browser.info",
    "https://at1.api.radio-browser.info",
    "https://nl1.api.radio-browser.info",
)
USER_AGENT = "RadioBalkan-CI/0.0 (+https://github.com/bren-wp/Radio-Balkan)"
REQUIRED = {"mp3", "aac", "https", "indirect"}
SSL_CONTEXT = ssl.create_default_context()


def _public_addresses(hostname: str) -> list[ipaddress.IPv4Address | ipaddress.IPv6Address]:
    host = (hostname or "").strip().rstrip(".")
    if not host:
        return []
    try:
        return [ipaddress.ip_address(host.split("%", 1)[0])]
    except ValueError:
        pass

    addresses: list[ipaddress.IPv4Address | ipaddress.IPv6Address] = []
    for info in socket.getaddrinfo(host, None, type=socket.SOCK_STREAM):
        raw = str(info[4][0]).split("%", 1)[0]
        try:
            address = ipaddress.ip_address(raw)
        except ValueError:
            continue
        if address not in addresses:
            addresses.append(address)
    return addresses


def _resolved_public_target(value: str) -> tuple[urllib.parse.ParseResult, str]:
    parsed = urllib.parse.urlparse(value)
    if parsed.scheme not in ("http", "https") or not parsed.hostname:
        raise ValueError("URL must use http/https and contain a host")
    if parsed.username is not None or parsed.password is not None:
        raise ValueError("credential-bearing URLs are not allowed")
    try:
        port = parsed.port
    except ValueError as exc:
        raise ValueError("invalid URL port") from exc
    if port is not None and not (1 <= port <= 65535):
        raise ValueError("invalid URL port")

    addresses = _public_addresses(parsed.hostname)
    if not addresses:
        raise ValueError("URL host did not resolve")
    unsafe = [str(address) for address in addresses if not address.is_global]
    if unsafe:
        raise ValueError("URL resolves to a non-public address: " + ",".join(unsafe))
    # The first validated public address is pinned into the actual socket
    # connection below. DNS is never consulted again between validation and
    # connect, which closes the DNS-rebinding validation/connection race.
    return parsed, str(addresses[0])


def validate_public_url(value: str) -> str:
    _resolved_public_target(value)
    return value


class PinnedHTTPConnection(http.client.HTTPConnection):
    def __init__(self, host, *, pinned_address: str, timeout=socket._GLOBAL_DEFAULT_TIMEOUT):
        super().__init__(host, timeout=timeout)
        self._pinned_address = pinned_address

    def connect(self):
        self.sock = socket.create_connection(
            (self._pinned_address, self.port),
            self.timeout,
            self.source_address,
        )
        if self._tunnel_host:
            self._tunnel()


class PinnedHTTPSConnection(http.client.HTTPSConnection):
    def __init__(
        self,
        host,
        *,
        pinned_address: str,
        timeout=socket._GLOBAL_DEFAULT_TIMEOUT,
        context: ssl.SSLContext,
    ):
        super().__init__(host, timeout=timeout, context=context)
        self._pinned_address = pinned_address

    def connect(self):
        sock = socket.create_connection(
            (self._pinned_address, self.port),
            self.timeout,
            self.source_address,
        )
        server_hostname = self.host
        if self._tunnel_host:
            self.sock = sock
            self._tunnel()
            sock = self.sock
            server_hostname = self._tunnel_host
        self.sock = self._context.wrap_socket(sock, server_hostname=server_hostname)


class PinnedHTTPHandler(urllib.request.HTTPHandler):
    def http_open(self, req):
        _, pinned_address = _resolved_public_target(req.full_url)
        return self.do_open(
            lambda host, timeout=socket._GLOBAL_DEFAULT_TIMEOUT: PinnedHTTPConnection(
                host,
                pinned_address=pinned_address,
                timeout=timeout,
            ),
            req,
        )


class PinnedHTTPSHandler(urllib.request.HTTPSHandler):
    def https_open(self, req):
        _, pinned_address = _resolved_public_target(req.full_url)
        return self.do_open(
            lambda host, timeout=socket._GLOBAL_DEFAULT_TIMEOUT: PinnedHTTPSConnection(
                host,
                pinned_address=pinned_address,
                timeout=timeout,
                context=self._context,
            ),
            req,
        )


class SafeRedirectHandler(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        target = urllib.parse.urljoin(req.full_url, newurl)
        validate_public_url(target)
        return super().redirect_request(req, fp, code, msg, headers, target)


SAFE_OPENER = urllib.request.build_opener(
    urllib.request.ProxyHandler({}),
    PinnedHTTPHandler(),
    PinnedHTTPSHandler(context=SSL_CONTEXT),
    SafeRedirectHandler(),
)


def request(url: str, timeout: float = 8.0, accept: str = "*/*"):
    validate_public_url(url)
    req = urllib.request.Request(
        url,
        headers={
            "User-Agent": USER_AGENT,
            "Accept": accept,
            "Icy-MetaData": "0",
            "Cache-Control": "no-cache",
            "Connection": "close",
        },
    )
    return SAFE_OPENER.open(req, timeout=timeout)


def get_json(url: str, timeout: float = 8.0):
    with request(url, timeout, "application/json") as response:
        raw = response.read(2 * 1024 * 1024)
    return json.loads(raw.decode("utf-8", "replace"))


def discover_servers() -> list[str]:
    servers: list[str] = []
    try:
        rows = get_json("https://all.api.radio-browser.info/json/servers", 7.0)
        for row in rows if isinstance(rows, list) else []:
            name = str(row.get("name", "")).strip()
            if not name:
                continue
            base = name if name.startswith(("http://", "https://")) else "https://" + name
            parsed = urllib.parse.urlparse(base)
            if parsed.scheme == "https" and parsed.hostname and parsed.hostname.endswith(".api.radio-browser.info"):
                servers.append(base.rstrip("/"))
    except Exception as exc:
        print(f"[live-radio] server discovery warning: {type(exc).__name__}: {exc}")

    for base in FALLBACK_SERVERS:
        if base not in servers:
            servers.append(base)
    return servers


def fetch_country(server: str, country: str) -> list[dict]:
    params = urllib.parse.urlencode(
        {
            "countrycode": country,
            "hidebroken": "true",
            "order": "votes",
            "reverse": "true",
            "limit": "100",
        }
    )
    rows = get_json(f"{server}/json/stations/search?{params}", 10.0)
    return rows if isinstance(rows, list) else []


def collect_catalog() -> tuple[str, list[dict]]:
    errors: list[str] = []
    for server in discover_servers():
        stations: list[dict] = []
        countries_seen: set[str] = set()
        try:
            for country in COUNTRIES:
                rows = fetch_country(server, country)
                if rows:
                    countries_seen.add(country)
                    stations.extend(rows)
            if len(countries_seen) >= 6 and stations:
                print(
                    f"[live-radio] catalog server={server} "
                    f"countries={','.join(sorted(countries_seen))} rows={len(stations)}"
                )
                return server, stations
            errors.append(f"{server}: only {len(countries_seen)} Balkan countries returned stations")
        except Exception as exc:
            errors.append(f"{server}: {type(exc).__name__}: {exc}")
    raise RuntimeError("Radio Browser catalog unavailable: " + " | ".join(errors[-4:]))


def station_caps(station: dict) -> set[str]:
    codec = str(station.get("codec", "")).strip().upper()
    resolved = str(station.get("url_resolved", "")).strip()
    raw = str(station.get("url", "")).strip()
    caps: set[str] = set()
    if "MP3" in codec or "MPEG" in codec:
        caps.add("mp3")
    if "AAC" in codec:
        caps.add("aac")
    if resolved.lower().startswith("https://"):
        caps.add("https")
    if raw and resolved and raw != resolved:
        caps.add("indirect")
    return caps


def usable_url(value: str) -> bool:
    try:
        parsed = urllib.parse.urlparse(value)
        return parsed.scheme in ("http", "https") and bool(parsed.hostname)
    except Exception:
        return False


def probe_stream(url: str) -> tuple[bool, str]:
    if not usable_url(url):
        return False, "invalid URL"
    started = time.monotonic()
    try:
        with request(
            url,
            8.0,
            "audio/*,application/ogg,application/vnd.apple.mpegurl,application/x-mpegurl,*/*;q=0.3",
        ) as response:
            status = getattr(response, "status", 200) or 200
            final_url = response.geturl()
            content_type = (response.headers.get("Content-Type") or "").lower()
            data = response.read(2048)
        elapsed = time.monotonic() - started
        if status < 200 or status >= 400:
            return False, f"HTTP {status}"
        if len(data) < 64:
            return False, f"only {len(data)} bytes"
        if "text/html" in content_type:
            return False, f"unexpected {content_type}"
        return True, f"{status} {content_type or 'unknown'} {len(data)}B {elapsed:.2f}s -> {final_url}"
    except Exception as exc:
        return False, f"{type(exc).__name__}: {exc}"


def playlist_targets(data: bytes, base_url: str) -> list[str]:
    text = data.decode("utf-8", "replace")
    targets: list[str] = []
    for raw_line in text.splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" in line and line.lower().startswith("file"):
            line = line.split("=", 1)[1].strip()
        candidate = urllib.parse.urljoin(base_url, line)
        if usable_url(candidate) and candidate not in targets:
            targets.append(candidate)
    return targets


def probe_indirect(raw_url: str) -> tuple[bool, str]:
    if not usable_url(raw_url):
        return False, "invalid raw URL"

    started = time.monotonic()

    def walk(url: str, depth: int, seen: set[str]) -> tuple[bool, str]:
        if depth > 2:
            return False, "playlist nesting limit exceeded"
        if url in seen:
            return False, "playlist loop detected"
        seen.add(url)
        try:
            with request(
                url,
                8.0,
                "audio/*,application/ogg,application/vnd.apple.mpegurl,application/x-mpegurl,audio/x-scpls,*/*;q=0.3",
            ) as response:
                status = getattr(response, "status", 200) or 200
                final_url = response.geturl()
                content_type = (response.headers.get("Content-Type") or "").lower()
                data = response.read(4096)
            if status < 200 or status >= 400:
                return False, f"HTTP {status}"
            if "text/html" in content_type:
                return False, f"unexpected {content_type}"

            lowered_path = urllib.parse.urlparse(final_url).path.lower()
            playlist = (
                "mpegurl" in content_type
                or "scpls" in content_type
                or lowered_path.endswith((".m3u", ".m3u8", ".pls"))
                or data.lstrip().startswith(b"#EXTM3U")
                or b"[playlist]" in data[:128].lower()
            )
            if playlist:
                targets = playlist_targets(data, final_url)
                if not targets:
                    return False, "playlist had no usable stream URL"
                failures: list[str] = []
                for target in targets[:6]:
                    ok, detail = walk(target, depth + 1, seen)
                    if ok:
                        return True, f"playlist -> {target} :: {detail}"
                    failures.append(detail)
                return False, "playlist targets failed: " + " | ".join(failures[-3:])

            if len(data) < 64:
                return False, f"only {len(data)} stream bytes"
            if final_url != url:
                return True, f"redirect -> {final_url} {content_type or 'unknown'} {len(data)}B"
            return True, f"stream {content_type or 'unknown'} {len(data)}B"
        except Exception as exc:
            return False, f"{type(exc).__name__}: {exc}"

    ok, detail = walk(raw_url, 0, set())
    elapsed = time.monotonic() - started
    return ok, f"{detail} total={elapsed:.2f}s"


def main() -> int:
    # Fail closed if the probe safety contract ever regresses. These checks are
    # deterministic and require no external network access.
    for unsafe_url in (
        "http://127.0.0.1/",
        "http://10.0.0.1/",
        "http://169.254.169.254/latest/meta-data/",
        "http://[::1]/",
    ):
        try:
            validate_public_url(unsafe_url)
        except ValueError:
            pass
        else:
            print(f"[live-radio] FAIL: unsafe probe URL accepted: {unsafe_url}", file=sys.stderr)
            return 1

    try:
        server, stations = collect_catalog()
    except Exception as exc:
        print(f"[live-radio] FAIL: {exc}", file=sys.stderr)
        return 1

    candidates: list[tuple[int, int, dict, set[str]]] = []
    for station in stations:
        resolved = str(station.get("url_resolved", "")).strip()
        caps = station_caps(station)
        if not usable_url(resolved) or not caps:
            continue
        last_ok = station.get("lastcheckok")
        if last_ok not in (1, True, "1", "true", "True"):
            continue
        votes = int(station.get("votes") or 0)
        candidates.append((len(caps), votes, station, caps))

    candidates.sort(key=lambda item: (item[0], item[1]), reverse=True)
    covered: set[str] = set()
    successful: list[str] = []
    tried_urls: set[tuple[str, str]] = set()
    attempts = 0

    # Greedily pick stations that cover currently missing capabilities. Then
    # continue through popular candidates so one stale Radio Browser record is
    # never enough to fail the build.
    while REQUIRED - covered and attempts < 24:
        missing = REQUIRED - covered
        choice = None
        choice_index = -1
        best = (-1, -1, -1)
        for idx, (cap_count, votes, station, caps) in enumerate(candidates):
            resolved = str(station.get("url_resolved", "")).strip()
            raw = str(station.get("url", "")).strip()
            attempt_key = (resolved, raw if "indirect" in caps else "")
            if attempt_key in tried_urls:
                continue
            gain = len(caps & missing)
            score = (gain, cap_count, votes)
            if gain and score > best:
                best = score
                choice = (station, caps)
                choice_index = idx
        if choice is None:
            break

        station, caps = choice
        resolved = str(station.get("url_resolved", "")).strip()
        raw = str(station.get("url", "")).strip()
        attempt_key = (resolved, raw if "indirect" in caps else "")
        tried_urls.add(attempt_key)
        attempts += 1
        ok, detail = probe_stream(resolved)
        name = str(station.get("name", "")).strip() or "(unnamed)"
        country = str(station.get("countrycode", "")).strip()
        codec = str(station.get("codec", "")).strip()
        if ok:
            verified_caps = set(caps)
            indirect_detail = ""
            if "indirect" in verified_caps and "indirect" in missing:
                indirect_ok, indirect_detail = probe_indirect(raw)
                if not indirect_ok:
                    verified_caps.discard("indirect")
                    print(
                        f"[live-radio] indirect retry {country} {name!r} raw={raw!r} "
                        f":: {indirect_detail}"
                    )
            gained = verified_caps & missing
            covered.update(verified_caps)
            successful.append(f"{country}:{name} [{codec}] => {','.join(sorted(verified_caps))}")
            suffix = f" :: indirect {indirect_detail}" if indirect_detail and "indirect" in verified_caps else ""
            print(
                f"[live-radio] PASS {country} {name!r} codec={codec} "
                f"caps={sorted(verified_caps)} :: {detail}{suffix}"
            )
        else:
            print(f"[live-radio] retry {country} {name!r} codec={codec} caps={sorted(caps)} :: {detail}")
            # Keep the failed row out of future selection.
            candidates.pop(choice_index)

    missing = REQUIRED - covered
    if missing:
        available: dict[str, int] = {cap: 0 for cap in REQUIRED}
        for _, _, _, caps in candidates:
            for cap in caps & REQUIRED:
                available[cap] += 1
        print(
            f"[live-radio] FAIL server={server} attempts={attempts} "
            f"covered={sorted(covered)} missing={sorted(missing)} available={available}",
            file=sys.stderr,
        )
        return 1

    countries = sorted({entry.split(":", 1)[0] for entry in successful})
    print(
        f"[live-radio] OK attempts={attempts} covered={sorted(covered)} "
        f"successful_countries={countries}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
