#!/usr/bin/env python3
# -*- coding: utf-8 -*-

from __future__ import annotations

import argparse
import json
import logging
import os
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any

from aliyunsdkcore.client import AcsClient
from aliyunsdkcore.request import CommonRequest
from aliyunsdkecs.request.v20140526 import (
    DescribeInstancesRequest,
    StartInstancesRequest,
    StopInstancesRequest,
)


LOGGER = logging.getLogger("cdt-guard")
TRANSIENT_STATUSES = {"Pending", "Starting", "Stopping"}
STOPPED_STATUSES = {"Stopped"}


@dataclass(frozen=True)
class Config:
    access_key_id: str
    access_key_secret: str
    region_id: str
    instance_ids: list[str]
    threshold_gb: float
    interval_seconds: int
    control_mode: str
    stopped_mode: str
    force_stop: bool
    dry_run: bool
    run_once: bool
    heartbeat_file: Path


def main() -> int:
    parser = argparse.ArgumentParser(description="CDT traffic guard for Aliyun ECS")
    parser.add_argument("--healthcheck", action="store_true", help="check loop heartbeat and exit")
    args = parser.parse_args()

    configure_logging()

    if args.healthcheck:
        return healthcheck()

    try:
        config = load_config()
    except ValueError as exc:
        LOGGER.error("invalid configuration: %s", exc)
        return 2

    LOGGER.info(
        "cdt guard started version=%s region=%s instances=%s threshold_gb=%.2f interval=%ss mode=%s dry_run=%s stopped_mode=%s force_stop=%s",
        os.getenv("CDT_GUARD_VERSION", "dev"),
        config.region_id,
        ",".join(config.instance_ids),
        config.threshold_gb,
        config.interval_seconds,
        config.control_mode,
        config.dry_run,
        config.stopped_mode,
        config.force_stop,
    )

    client = AcsClient(config.access_key_id, config.access_key_secret, config.region_id)

    while True:
        started_at = time.time()
        run_cycle(client, config)
        write_heartbeat(config.heartbeat_file)

        if config.run_once:
            LOGGER.info("run once enabled, exiting")
            return 0

        elapsed = time.time() - started_at
        sleep_seconds = max(1, config.interval_seconds - int(elapsed))
        LOGGER.info("next check in %s seconds", sleep_seconds)
        time.sleep(sleep_seconds)


def configure_logging() -> None:
    level_name = os.getenv("LOG_LEVEL", "INFO").upper()
    logging.basicConfig(
        level=getattr(logging, level_name, logging.INFO),
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
        stream=sys.stdout,
    )


def load_config() -> Config:
    access_key_id = require_env("ALIYUN_ACCESS_KEY_ID")
    access_key_secret = require_env("ALIYUN_ACCESS_KEY_SECRET")
    region_id = require_env("ALIYUN_REGION_ID")
    instance_ids = parse_instance_ids()

    threshold_gb = parse_float_env("CDT_TRAFFIC_THRESHOLD_GB", 180.0)
    if threshold_gb <= 0:
        raise ValueError("CDT_TRAFFIC_THRESHOLD_GB must be greater than 0")

    interval_seconds = parse_int_env("CDT_CHECK_INTERVAL_SECONDS", 60)
    if interval_seconds < 10:
        raise ValueError("CDT_CHECK_INTERVAL_SECONDS must be at least 10")

    control_mode = os.getenv("CDT_CONTROL_MODE", "keep_running").strip().lower()
    if control_mode not in {"keep_running", "protect_only", "dry_run"}:
        raise ValueError("CDT_CONTROL_MODE must be keep_running, protect_only, or dry_run")

    stopped_mode = os.getenv("ECS_STOPPED_MODE", "StopCharging").strip()
    if stopped_mode not in {"KeepCharging", "StopCharging"}:
        raise ValueError("ECS_STOPPED_MODE must be KeepCharging or StopCharging")

    dry_run = parse_bool_env("CDT_DRY_RUN", False) or control_mode == "dry_run"

    return Config(
        access_key_id=access_key_id,
        access_key_secret=access_key_secret,
        region_id=region_id,
        instance_ids=instance_ids,
        threshold_gb=threshold_gb,
        interval_seconds=interval_seconds,
        control_mode=control_mode,
        stopped_mode=stopped_mode,
        force_stop=parse_bool_env("ECS_FORCE_STOP", False),
        dry_run=dry_run,
        run_once=parse_bool_env("RUN_ONCE", False),
        heartbeat_file=Path(os.getenv("CDT_HEARTBEAT_FILE", "/tmp/cdt-guard-heartbeat")),
    )


def require_env(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        raise ValueError(f"{name} is required")
    return value


def parse_instance_ids() -> list[str]:
    raw = os.getenv("ECS_INSTANCE_IDS", "").strip() or os.getenv("ECS_INSTANCE_ID", "").strip()
    if not raw:
        raise ValueError("ECS_INSTANCE_ID or ECS_INSTANCE_IDS is required")

    instance_ids = [item.strip() for item in raw.split(",") if item.strip()]
    if not instance_ids:
        raise ValueError("ECS_INSTANCE_ID or ECS_INSTANCE_IDS is empty")
    return instance_ids


def parse_float_env(name: str, default: float) -> float:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return float(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be a number") from exc


def parse_int_env(name: str, default: int) -> int:
    raw = os.getenv(name, "").strip()
    if not raw:
        return default
    try:
        return int(raw)
    except ValueError as exc:
        raise ValueError(f"{name} must be an integer") from exc


def parse_bool_env(name: str, default: bool) -> bool:
    raw = os.getenv(name, "").strip().lower()
    if not raw:
        return default
    return raw in {"1", "true", "yes", "y", "on"}


def run_cycle(client: AcsClient, config: Config) -> None:
    try:
        total_gb = get_total_traffic_gb(client)
    except Exception:
        LOGGER.exception("failed to fetch CDT traffic; skip this cycle")
        return

    over_threshold = total_gb >= config.threshold_gb
    LOGGER.info(
        "traffic check total_gb=%.2f threshold_gb=%.2f over_threshold=%s",
        total_gb,
        config.threshold_gb,
        over_threshold,
    )

    for instance_id in config.instance_ids:
        handle_instance(client, config, instance_id, total_gb, over_threshold)


def get_total_traffic_gb(client: AcsClient) -> float:
    request = CommonRequest()
    request.set_domain("cdt.aliyuncs.com")
    request.set_version("2021-08-13")
    request.set_action_name("ListCdtInternetTraffic")
    request.set_method("POST")

    response = client.do_action_with_exception(request)
    payload = json.loads(response.decode("utf-8"))

    total_bytes = 0.0
    for detail in payload.get("TrafficDetails", []):
        total_bytes += parse_traffic_value(detail.get("Traffic", 0))

    return total_bytes / (1024**3)


def parse_traffic_value(value: Any) -> float:
    if value is None:
        return 0.0
    try:
        return float(value)
    except (TypeError, ValueError):
        LOGGER.warning("ignore invalid traffic value: %r", value)
        return 0.0


def handle_instance(
    client: AcsClient,
    config: Config,
    instance_id: str,
    total_gb: float,
    over_threshold: bool,
) -> None:
    status = get_ecs_status(client, instance_id)
    if status is None:
        LOGGER.error("skip instance because status is unavailable instance_id=%s", instance_id)
        return

    if over_threshold:
        LOGGER.info(
            "traffic %.2f GB >= threshold %.2f GB; protect instance_id=%s status=%s",
            total_gb,
            config.threshold_gb,
            instance_id,
            status,
        )
        stop_instance_if_needed(client, config, instance_id, status)
        return

    if config.control_mode == "keep_running":
        LOGGER.info(
            "traffic %.2f GB < threshold %.2f GB; keep instance running instance_id=%s status=%s",
            total_gb,
            config.threshold_gb,
            instance_id,
            status,
        )
        start_instance_if_needed(client, config, instance_id, status)
    else:
        LOGGER.info(
            "traffic %.2f GB < threshold %.2f GB; no start action in mode=%s instance_id=%s status=%s",
            total_gb,
            config.threshold_gb,
            config.control_mode,
            instance_id,
            status,
        )


def get_ecs_status(client: AcsClient, instance_id: str) -> str | None:
    try:
        request = DescribeInstancesRequest.DescribeInstancesRequest()
        request.set_InstanceIds(json.dumps([instance_id]))
        response = client.do_action_with_exception(request)
        payload = json.loads(response.decode("utf-8"))
    except Exception:
        LOGGER.exception("failed to describe ECS instance instance_id=%s", instance_id)
        return None

    instances = payload.get("Instances", {}).get("Instance", [])
    if not instances:
        LOGGER.error("ECS instance not found instance_id=%s", instance_id)
        return None

    status = str(instances[0].get("Status", "Unknown"))
    LOGGER.info("ECS status instance_id=%s status=%s", instance_id, status)
    return status


def start_instance_if_needed(client: AcsClient, config: Config, instance_id: str, status: str) -> None:
    if status == "Running":
        LOGGER.info("instance already running instance_id=%s", instance_id)
        return
    if status in TRANSIENT_STATUSES:
        LOGGER.info("instance is transient; skip start instance_id=%s status=%s", instance_id, status)
        return
    if status not in STOPPED_STATUSES:
        LOGGER.warning("instance status is not startable; skip start instance_id=%s status=%s", instance_id, status)
        return

    if config.dry_run:
        LOGGER.info("dry run: would start instance_id=%s", instance_id)
        return

    try:
        request = StartInstancesRequest.StartInstancesRequest()
        request.set_InstanceIds([instance_id])
        request.set_accept_format("json")
        response = client.do_action_with_exception(request)
        LOGGER.info("start instance submitted instance_id=%s response=%s", instance_id, response.decode("utf-8"))
    except Exception:
        LOGGER.exception("failed to start instance instance_id=%s", instance_id)


def stop_instance_if_needed(client: AcsClient, config: Config, instance_id: str, status: str) -> None:
    if status == "Stopped":
        LOGGER.info("instance already stopped instance_id=%s", instance_id)
        return
    if status in TRANSIENT_STATUSES:
        LOGGER.info("instance is transient; skip stop instance_id=%s status=%s", instance_id, status)
        return

    if config.dry_run:
        LOGGER.info(
            "dry run: would stop instance_id=%s stopped_mode=%s force_stop=%s",
            instance_id,
            config.stopped_mode,
            config.force_stop,
        )
        return

    try:
        request = StopInstancesRequest.StopInstancesRequest()
        request.set_InstanceIds([instance_id])
        request.set_ForceStop(config.force_stop)
        request.set_StoppedMode(config.stopped_mode)
        request.set_accept_format("json")
        response = client.do_action_with_exception(request)
        LOGGER.info(
            "stop instance submitted instance_id=%s stopped_mode=%s force_stop=%s response=%s",
            instance_id,
            config.stopped_mode,
            config.force_stop,
            response.decode("utf-8"),
        )
    except Exception:
        LOGGER.exception("failed to stop instance instance_id=%s", instance_id)


def write_heartbeat(path: Path) -> None:
    try:
        path.write_text(str(int(time.time())), encoding="utf-8")
    except Exception:
        LOGGER.exception("failed to write heartbeat path=%s", path)


def healthcheck() -> int:
    heartbeat_file = Path(os.getenv("CDT_HEARTBEAT_FILE", "/tmp/cdt-guard-heartbeat"))
    interval_seconds = parse_int_env("CDT_CHECK_INTERVAL_SECONDS", 60)
    max_age = max(interval_seconds * 3, 300)

    try:
        stat = heartbeat_file.stat()
    except FileNotFoundError:
        print(f"heartbeat file not found: {heartbeat_file}", file=sys.stderr)
        return 1

    age = time.time() - stat.st_mtime
    if age > max_age:
        print(f"heartbeat too old: age={age:.0f}s max_age={max_age}s", file=sys.stderr)
        return 1

    print(f"ok heartbeat_age={age:.0f}s")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
