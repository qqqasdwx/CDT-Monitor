#!/usr/bin/env python3
# -*- coding: utf-8 -*-

from __future__ import annotations

import argparse
import json
import logging
import math
import os
import sys
import time
from dataclasses import dataclass
from pathlib import Path
from typing import Any
from urllib import error as urllib_error
from urllib import parse as urllib_parse
from urllib import request as urllib_request

from aliyunsdkcore.client import AcsClient
from aliyunsdkcore.http import protocol_type
from aliyunsdkcore.request import CommonRequest
from aliyunsdkecs.request.v20140526 import (
    DescribeInstancesRequest,
    StartInstancesRequest,
    StopInstancesRequest,
)


LOGGER = logging.getLogger("cdt-guard")
TRANSIENT_STATUSES = {"Pending", "Starting", "Stopping"}
STOPPED_STATUSES = {"Stopped"}
ECS_STATUS_LABELS = {
    "Pending": "等待中",
    "Starting": "启动中",
    "Running": "运行中",
    "Stopping": "停止中",
    "Stopped": "已停止",
}
KNOWN_ECS_STATUSES = set(ECS_STATUS_LABELS)
CONTROL_MODE_LABELS = {
    "keep_running": "保活",
    "protect_only": "仅保护",
    "dry_run": "模拟运行",
}
KUMA_PUSH_TIMEOUT_SECONDS = 10
KUMA_MESSAGE_MAX_LENGTH = 1000


@dataclass(frozen=True)
class Config:
    access_key_id: str
    access_key_secret: str
    region_id: str
    instance_id: str
    threshold_gb: float
    interval_seconds: int
    control_mode: str
    stopped_mode: str
    force_stop: bool
    dry_run: bool
    run_once: bool
    heartbeat_file: Path
    kuma_push_url: str | None


@dataclass(frozen=True)
class Result:
    success: bool
    message: str


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
        send_kuma_push(
            os.getenv("UPTIME_KUMA_PUSH_URL", "").strip() or None,
            Result(False, "配置错误，请查看容器日志"),
        )
        return 2

    LOGGER.info(
        "cdt guard started version=%s region=%s instance=%s threshold_gb=%.2f interval=%ss mode=%s dry_run=%s stopped_mode=%s force_stop=%s",
        os.getenv("CDT_GUARD_VERSION", "dev"),
        config.region_id,
        config.instance_id,
        config.threshold_gb,
        config.interval_seconds,
        config.control_mode,
        config.dry_run,
        config.stopped_mode,
        config.force_stop,
    )

    try:
        client = AcsClient(config.access_key_id, config.access_key_secret, config.region_id)
    except Exception as exc:
        LOGGER.exception("failed to initialize Aliyun client")
        send_kuma_push(
            config.kuma_push_url,
            Result(False, f"阿里云客户端初始化失败（{error_label(exc)}）"),
        )
        return 1

    while True:
        started_at = time.monotonic()
        try:
            result = run_cycle(client, config)
        except Exception as exc:
            LOGGER.exception("unexpected cycle failure")
            result = Result(False, f"巡检发生未预期错误（{error_label(exc)}）")

        elapsed_ms = max(0, round((time.monotonic() - started_at) * 1000))
        if result.success and not write_heartbeat(config.heartbeat_file):
            result = Result(False, f"{result.message}；心跳文件写入失败")

        send_kuma_push(config.kuma_push_url, result, elapsed_ms)

        if config.run_once:
            exit_code = 0 if result.success else 1
            LOGGER.info("run once enabled, exiting code=%s", exit_code)
            return exit_code

        elapsed = time.monotonic() - started_at
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
    instance_id = require_env("ECS_INSTANCE_ID")

    threshold_gb = parse_float_env("CDT_TRAFFIC_THRESHOLD_GB", 180.0)
    if not math.isfinite(threshold_gb) or threshold_gb <= 0:
        raise ValueError("CDT_TRAFFIC_THRESHOLD_GB must be a finite number greater than 0")

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
    kuma_push_url = os.getenv("UPTIME_KUMA_PUSH_URL", "").strip() or None
    if kuma_push_url is not None:
        validate_http_url("UPTIME_KUMA_PUSH_URL", kuma_push_url)

    return Config(
        access_key_id=access_key_id,
        access_key_secret=access_key_secret,
        region_id=region_id,
        instance_id=instance_id,
        threshold_gb=threshold_gb,
        interval_seconds=interval_seconds,
        control_mode=control_mode,
        stopped_mode=stopped_mode,
        force_stop=parse_bool_env("ECS_FORCE_STOP", False),
        dry_run=dry_run,
        run_once=parse_bool_env("RUN_ONCE", False),
        heartbeat_file=Path(os.getenv("CDT_HEARTBEAT_FILE", "/tmp/cdt-guard-heartbeat")),
        kuma_push_url=kuma_push_url,
    )


def require_env(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        raise ValueError(f"{name} is required")
    return value


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
    if raw in {"1", "true", "yes", "y", "on"}:
        return True
    if raw in {"0", "false", "no", "n", "off"}:
        return False
    raise ValueError(f"{name} must be a boolean")


def validate_http_url(name: str, value: str) -> None:
    parsed = urllib_parse.urlsplit(value)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        raise ValueError(f"{name} must be a valid HTTP or HTTPS URL")


def run_cycle(client: AcsClient, config: Config) -> Result:
    try:
        total_gb = get_total_traffic_gb(client)
    except Exception as exc:
        LOGGER.exception("failed to fetch CDT traffic; skip this cycle")
        return Result(False, f"CDT 流量查询失败（{error_label(exc)}）")

    over_threshold = total_gb >= config.threshold_gb
    LOGGER.info(
        "traffic check total_gb=%.2f threshold_gb=%.2f over_threshold=%s",
        total_gb,
        config.threshold_gb,
        over_threshold,
    )

    instance_result = handle_instance(client, config, config.instance_id, total_gb, over_threshold)
    message = f"流量={total_gb:.2f}GB，阈值={config.threshold_gb:.2f}GB；{instance_result.message}"
    return Result(instance_result.success, message)


def get_total_traffic_gb(client: AcsClient) -> float:
    request = CommonRequest()
    request.set_domain("cdt.aliyuncs.com")
    request.set_version("2021-08-13")
    request.set_action_name("ListCdtInternetTraffic")
    request.set_method("POST")
    request.set_protocol_type(protocol_type.HTTPS)

    response = client.do_action_with_exception(request)
    payload = decode_json_object(response, "CDT traffic")

    if "TrafficDetails" not in payload:
        raise ValueError("CDT response is missing TrafficDetails")
    traffic_details = payload["TrafficDetails"]
    if not isinstance(traffic_details, list):
        raise ValueError("CDT TrafficDetails must be a list")

    total_bytes = 0.0
    for detail in traffic_details:
        if not isinstance(detail, dict) or "Traffic" not in detail:
            raise ValueError("CDT traffic detail is invalid")
        total_bytes += parse_traffic_value(detail["Traffic"])

    if not math.isfinite(total_bytes) or total_bytes < 0:
        raise ValueError("CDT total traffic is invalid")

    return total_bytes / (1024**3)


def parse_traffic_value(value: Any) -> float:
    if value is None or isinstance(value, bool):
        raise ValueError("CDT traffic value is invalid")
    try:
        parsed = float(value)
    except (TypeError, ValueError):
        raise ValueError("CDT traffic value is not numeric") from None
    if not math.isfinite(parsed) or parsed < 0:
        raise ValueError("CDT traffic value must be finite and non-negative")
    return parsed


def handle_instance(
    client: AcsClient,
    config: Config,
    instance_id: str,
    total_gb: float,
    over_threshold: bool,
) -> Result:
    try:
        status = get_ecs_status(client, instance_id)
    except Exception as exc:
        LOGGER.exception("failed to describe ECS instance instance_id=%s", instance_id)
        return Result(False, f"实例={instance_id}，状态查询失败（{error_label(exc)}）")

    if over_threshold:
        LOGGER.info(
            "traffic %.2f GB >= threshold %.2f GB; protect instance_id=%s status=%s",
            total_gb,
            config.threshold_gb,
            instance_id,
            status,
        )
        return stop_instance_if_needed(client, config, instance_id, status)

    if config.control_mode == "keep_running":
        LOGGER.info(
            "traffic %.2f GB < threshold %.2f GB; keep instance running instance_id=%s status=%s",
            total_gb,
            config.threshold_gb,
            instance_id,
            status,
        )
        return start_instance_if_needed(client, config, instance_id, status)
    else:
        LOGGER.info(
            "traffic %.2f GB < threshold %.2f GB; no start action in mode=%s instance_id=%s status=%s",
            total_gb,
            config.threshold_gb,
            config.control_mode,
            instance_id,
            status,
        )
        return Result(
            True,
            f"实例={instance_id}，状态={ecs_status_label(status)}，操作=无需处理，模式={control_mode_label(config.control_mode)}",
        )


def get_ecs_status(client: AcsClient, instance_id: str) -> str:
    request = DescribeInstancesRequest.DescribeInstancesRequest()
    request.set_InstanceIds(json.dumps([instance_id]))
    request.set_protocol_type(protocol_type.HTTPS)
    response = client.do_action_with_exception(request)
    payload = decode_json_object(response, "ECS describe")

    instances_container = payload.get("Instances")
    if not isinstance(instances_container, dict):
        raise ValueError("ECS response is missing Instances")
    instances = instances_container.get("Instance")
    if not isinstance(instances, list) or not instances or not isinstance(instances[0], dict):
        raise ValueError(f"ECS instance not found: {instance_id}")

    instance = instances[0]
    if instance.get("InstanceId") != instance_id:
        raise ValueError(f"ECS returned an unexpected instance for {instance_id}")
    status = instance.get("Status")
    if not isinstance(status, str) or status not in KNOWN_ECS_STATUSES:
        raise ValueError(f"ECS returned an invalid status for {instance_id}")
    LOGGER.info("ECS status instance_id=%s status=%s", instance_id, status)
    return status


def start_instance_if_needed(
    client: AcsClient,
    config: Config,
    instance_id: str,
    status: str,
) -> Result:
    if status == "Running":
        LOGGER.info("instance already running instance_id=%s", instance_id)
        return Result(True, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=无需处理")
    if status in TRANSIENT_STATUSES:
        LOGGER.info("instance is transient; skip start instance_id=%s status=%s", instance_id, status)
        return Result(True, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=等待状态稳定")
    if status not in STOPPED_STATUSES:
        LOGGER.warning("instance status is not startable; skip start instance_id=%s status=%s", instance_id, status)
        return Result(False, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=无法启动")

    if config.dry_run:
        LOGGER.info("dry run: would start instance_id=%s", instance_id)
        return Result(True, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=模拟启动")

    try:
        request = StartInstancesRequest.StartInstancesRequest()
        request.set_InstanceIds([instance_id])
        request.set_accept_format("json")
        request.set_protocol_type(protocol_type.HTTPS)
        response = client.do_action_with_exception(request)
        request_id = parse_action_request_id(response, "ECS start")
        LOGGER.info("start instance submitted instance_id=%s request_id=%s", instance_id, request_id)
        return Result(True, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=已提交启动")
    except Exception as exc:
        LOGGER.exception("failed to start instance instance_id=%s", instance_id)
        return Result(
            False,
            f"实例={instance_id}，状态={ecs_status_label(status)}，启动失败（{error_label(exc)}）",
        )


def stop_instance_if_needed(
    client: AcsClient,
    config: Config,
    instance_id: str,
    status: str,
) -> Result:
    if status == "Stopped":
        LOGGER.info("instance already stopped instance_id=%s", instance_id)
        return Result(True, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=无需处理")
    if status in TRANSIENT_STATUSES:
        LOGGER.info("instance is transient; skip stop instance_id=%s status=%s", instance_id, status)
        return Result(True, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=等待状态稳定")
    if status != "Running":
        LOGGER.warning("instance status is not stoppable; skip stop instance_id=%s status=%s", instance_id, status)
        return Result(False, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=无法停止")

    if config.dry_run:
        LOGGER.info(
            "dry run: would stop instance_id=%s stopped_mode=%s force_stop=%s",
            instance_id,
            config.stopped_mode,
            config.force_stop,
        )
        return Result(True, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=模拟停止")

    try:
        request = StopInstancesRequest.StopInstancesRequest()
        request.set_InstanceIds([instance_id])
        request.set_ForceStop(config.force_stop)
        request.set_StoppedMode(config.stopped_mode)
        request.set_accept_format("json")
        request.set_protocol_type(protocol_type.HTTPS)
        response = client.do_action_with_exception(request)
        request_id = parse_action_request_id(response, "ECS stop")
        LOGGER.info(
            "stop instance submitted instance_id=%s stopped_mode=%s force_stop=%s request_id=%s",
            instance_id,
            config.stopped_mode,
            config.force_stop,
            request_id,
        )
        return Result(True, f"实例={instance_id}，状态={ecs_status_label(status)}，操作=已提交停止")
    except Exception as exc:
        LOGGER.exception("failed to stop instance instance_id=%s", instance_id)
        return Result(
            False,
            f"实例={instance_id}，状态={ecs_status_label(status)}，停止失败（{error_label(exc)}）",
        )


def decode_json_object(response: bytes, operation: str) -> dict[str, Any]:
    try:
        payload = json.loads(response.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise ValueError(f"{operation} response is not valid JSON") from exc
    if not isinstance(payload, dict):
        raise ValueError(f"{operation} response must be a JSON object")
    return payload


def parse_action_request_id(response: bytes, operation: str) -> str:
    payload = decode_json_object(response, operation)
    request_id = payload.get("RequestId")
    if not isinstance(request_id, str) or not request_id.strip():
        raise ValueError(f"{operation} response is missing RequestId")
    return request_id


def error_label(exc: Exception) -> str:
    code = getattr(exc, "error_code", None)
    if code:
        return f"{type(exc).__name__}:{str(code)[:80]}"
    return type(exc).__name__


def ecs_status_label(status: str) -> str:
    return ECS_STATUS_LABELS.get(status, status)


def control_mode_label(mode: str) -> str:
    return CONTROL_MODE_LABELS.get(mode, mode)


def write_heartbeat(path: Path) -> bool:
    try:
        path.write_text(str(int(time.time())), encoding="utf-8")
        return True
    except Exception:
        LOGGER.exception("failed to write heartbeat path=%s", path)
        return False


def send_kuma_push(push_url: str | None, result: Result, elapsed_ms: int | None = None) -> bool:
    if not push_url:
        return True

    try:
        validate_http_url("UPTIME_KUMA_PUSH_URL", push_url)
        parsed = urllib_parse.urlsplit(push_url)
        query = dict(urllib_parse.parse_qsl(parsed.query, keep_blank_values=True))
        query["status"] = "up" if result.success else "down"
        query["msg"] = result.message[:KUMA_MESSAGE_MAX_LENGTH]
        if elapsed_ms is not None:
            query["ping"] = str(elapsed_ms)
        target = urllib_parse.urlunsplit(parsed._replace(query=urllib_parse.urlencode(query)))
        request = urllib_request.Request(target, headers={"User-Agent": "cdt-guard"})
        with urllib_request.urlopen(request, timeout=KUMA_PUSH_TIMEOUT_SECONDS) as response:
            status_code = response.getcode()
            if status_code < 200 or status_code >= 300:
                raise RuntimeError(f"unexpected HTTP status {status_code}")
        LOGGER.info("Uptime Kuma push sent status=%s", query["status"])
        return True
    except urllib_error.HTTPError as exc:
        LOGGER.error("Uptime Kuma push failed http_status=%s", exc.code)
    except urllib_error.URLError as exc:
        LOGGER.error("Uptime Kuma push failed reason=%s", exc.reason)
    except Exception as exc:
        LOGGER.error("Uptime Kuma push failed error=%s", error_label(exc))
    return False


def healthcheck() -> int:
    heartbeat_file = Path(os.getenv("CDT_HEARTBEAT_FILE", "/tmp/cdt-guard-heartbeat"))
    try:
        interval_seconds = parse_int_env("CDT_CHECK_INTERVAL_SECONDS", 60)
    except ValueError as exc:
        print(f"invalid healthcheck configuration: {exc}", file=sys.stderr)
        return 1
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
