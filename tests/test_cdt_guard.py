from __future__ import annotations

import json
import os
import sys
import tempfile
import unittest
from pathlib import Path
from unittest import mock
from urllib import parse as urllib_parse


ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))

import cdt_guard  # noqa: E402


class FakeClient:
    def __init__(self, *responses: bytes | Exception):
        self.responses = list(responses)
        self.requests = []

    def do_action_with_exception(self, request):
        self.requests.append(request)
        response = self.responses.pop(0)
        if isinstance(response, Exception):
            raise response
        return response


def json_response(payload: object) -> bytes:
    return json.dumps(payload).encode("utf-8")


def make_config(**overrides) -> cdt_guard.Config:
    values = {
        "access_key_id": "test-ak",
        "access_key_secret": "test-secret",
        "region_id": "cn-hongkong",
        "instance_id": "i-test",
        "threshold_gb": 180.0,
        "interval_seconds": 60,
        "control_mode": "keep_running",
        "stopped_mode": "StopCharging",
        "force_stop": False,
        "dry_run": False,
        "run_once": True,
        "heartbeat_file": Path("/tmp/cdt-guard-test-heartbeat"),
        "kuma_push_url": None,
    }
    values.update(overrides)
    return cdt_guard.Config(**values)


class ConfigTests(unittest.TestCase):
    def setUp(self):
        self.environment = {
            "ALIYUN_ACCESS_KEY_ID": "test-ak",
            "ALIYUN_ACCESS_KEY_SECRET": "test-secret",
            "ALIYUN_REGION_ID": "cn-hongkong",
            "ECS_INSTANCE_ID": "i-test",
        }

    def test_load_config_defaults_to_keep_running_with_one_instance(self):
        with mock.patch.dict(os.environ, self.environment, clear=True):
            config = cdt_guard.load_config()

        self.assertEqual("keep_running", config.control_mode)
        self.assertEqual("i-test", config.instance_id)

    def test_load_config_does_not_accept_multi_instance_variable(self):
        environment = {**self.environment, "ECS_INSTANCE_ID": "", "ECS_INSTANCE_IDS": "i-one,i-two"}
        with mock.patch.dict(os.environ, environment, clear=True):
            with self.assertRaisesRegex(ValueError, "ECS_INSTANCE_ID is required"):
                cdt_guard.load_config()

    def test_load_config_rejects_non_finite_threshold(self):
        environment = {**self.environment, "CDT_TRAFFIC_THRESHOLD_GB": "nan"}
        with mock.patch.dict(os.environ, environment, clear=True):
            with self.assertRaisesRegex(ValueError, "finite number"):
                cdt_guard.load_config()

    def test_load_config_rejects_invalid_boolean(self):
        environment = {**self.environment, "ECS_FORCE_STOP": "treu"}
        with mock.patch.dict(os.environ, environment, clear=True):
            with self.assertRaisesRegex(ValueError, "must be a boolean"):
                cdt_guard.load_config()

    def test_load_config_accepts_one_kuma_push_url(self):
        environment = {
            **self.environment,
            "UPTIME_KUMA_PUSH_URL": "https://kuma.example/api/push/token?status=up&msg=OK&ping=",
        }
        with mock.patch.dict(os.environ, environment, clear=True):
            config = cdt_guard.load_config()

        self.assertEqual(environment["UPTIME_KUMA_PUSH_URL"], config.kuma_push_url)


class AliyunResponseTests(unittest.TestCase):
    def test_traffic_request_uses_https_and_sums_account_traffic(self):
        gib = 1024**3
        client = FakeClient(
            json_response({"TrafficDetails": [{"Traffic": gib}, {"Traffic": str(2 * gib)}]})
        )

        total_gb = cdt_guard.get_total_traffic_gb(client)

        self.assertEqual(3.0, total_gb)
        self.assertEqual("https", client.requests[0].get_protocol_type())

    def test_missing_traffic_details_is_a_failure(self):
        client = FakeClient(json_response({"RequestId": "request-id"}))

        with self.assertRaisesRegex(ValueError, "missing TrafficDetails"):
            cdt_guard.get_total_traffic_gb(client)

    def test_invalid_traffic_value_is_a_failure(self):
        client = FakeClient(json_response({"TrafficDetails": [{"Traffic": "nan"}]}))

        with self.assertRaisesRegex(ValueError, "finite"):
            cdt_guard.get_total_traffic_gb(client)

    def test_ecs_status_request_uses_https_and_validates_status(self):
        client = FakeClient(
            json_response({"Instances": {"Instance": [{"InstanceId": "i-test", "Status": "Running"}]}})
        )

        status = cdt_guard.get_ecs_status(client, "i-test")

        self.assertEqual("Running", status)
        self.assertEqual("https", client.requests[0].get_protocol_type())

    def test_unknown_ecs_status_is_a_failure(self):
        client = FakeClient(
            json_response({"Instances": {"Instance": [{"InstanceId": "i-test", "Status": "Unknown"}]}})
        )

        with self.assertRaisesRegex(ValueError, "invalid status"):
            cdt_guard.get_ecs_status(client, "i-test")


class CycleTests(unittest.TestCase):
    def test_keep_running_starts_a_stopped_instance(self):
        client = FakeClient(
            json_response({"TrafficDetails": [{"Traffic": 10 * 1024**3}]}),
            json_response({"Instances": {"Instance": [{"InstanceId": "i-test", "Status": "Stopped"}]}}),
            json_response({"RequestId": "start-request"}),
        )

        result = cdt_guard.run_cycle(client, make_config())

        self.assertTrue(result.success)
        self.assertIn("操作=已提交启动", result.message)
        self.assertEqual("https", client.requests[2].get_protocol_type())

    def test_over_threshold_stops_a_running_instance(self):
        client = FakeClient(
            json_response({"TrafficDetails": [{"Traffic": 190 * 1024**3}]}),
            json_response({"Instances": {"Instance": [{"InstanceId": "i-test", "Status": "Running"}]}}),
            json_response({"RequestId": "stop-request"}),
        )

        result = cdt_guard.run_cycle(client, make_config())

        self.assertTrue(result.success)
        self.assertIn("操作=已提交停止", result.message)
        self.assertEqual("https", client.requests[2].get_protocol_type())

    def test_invalid_traffic_skips_all_ecs_requests(self):
        client = FakeClient(json_response({"RequestId": "missing-details"}))

        result = cdt_guard.run_cycle(client, make_config())

        self.assertFalse(result.success)
        self.assertIn("CDT 流量查询失败", result.message)
        self.assertEqual(1, len(client.requests))

    def test_action_failure_marks_cycle_down(self):
        client = FakeClient(
            json_response({"TrafficDetails": [{"Traffic": 190 * 1024**3}]}),
            json_response({"Instances": {"Instance": [{"InstanceId": "i-test", "Status": "Running"}]}}),
            RuntimeError("stop failed"),
        )

        result = cdt_guard.run_cycle(client, make_config())

        self.assertFalse(result.success)
        self.assertIn("停止失败（RuntimeError）", result.message)


class KumaPushTests(unittest.TestCase):
    def test_push_reuses_one_url_and_replaces_status_message_and_ping(self):
        response = mock.MagicMock()
        response.getcode.return_value = 200
        response.__enter__.return_value = response

        with mock.patch.object(cdt_guard.urllib_request, "urlopen", return_value=response) as urlopen:
            success = cdt_guard.send_kuma_push(
                "https://kuma.example/api/push/token?status=up&msg=OK&ping=",
                cdt_guard.Result(False, "CDT 流量查询失败"),
                123,
            )

        self.assertTrue(success)
        request = urlopen.call_args.args[0]
        parsed = urllib_parse.urlsplit(request.full_url)
        query = dict(urllib_parse.parse_qsl(parsed.query))
        self.assertEqual("/api/push/token", parsed.path)
        self.assertEqual("down", query["status"])
        self.assertEqual("CDT 流量查询失败", query["msg"])
        self.assertEqual("123", query["ping"])
        self.assertEqual(1, urlopen.call_count)

    def test_no_push_url_is_a_noop(self):
        with mock.patch.object(cdt_guard.urllib_request, "urlopen") as urlopen:
            success = cdt_guard.send_kuma_push(None, cdt_guard.Result(True, "ok"), 1)

        self.assertTrue(success)
        urlopen.assert_not_called()


class MainLoopTests(unittest.TestCase):
    def test_failed_run_once_pushes_one_down_result_and_returns_failure(self):
        config = make_config(kuma_push_url="https://kuma.example/api/push/token")
        failed_result = cdt_guard.Result(False, "CDT 流量查询失败")

        with (
            mock.patch.object(cdt_guard, "load_config", return_value=config),
            mock.patch.object(cdt_guard, "AcsClient"),
            mock.patch.object(cdt_guard, "run_cycle", return_value=failed_result),
            mock.patch.object(cdt_guard, "write_heartbeat") as write_heartbeat,
            mock.patch.object(cdt_guard, "send_kuma_push", return_value=True) as send_kuma_push,
            mock.patch.object(sys, "argv", ["cdt_guard.py"]),
        ):
            exit_code = cdt_guard.main()

        self.assertEqual(1, exit_code)
        write_heartbeat.assert_not_called()
        send_kuma_push.assert_called_once()
        self.assertEqual(failed_result, send_kuma_push.call_args.args[1])


class HealthcheckTests(unittest.TestCase):
    def test_recent_success_heartbeat_is_healthy(self):
        with tempfile.TemporaryDirectory() as directory:
            heartbeat = Path(directory) / "heartbeat"
            self.assertTrue(cdt_guard.write_heartbeat(heartbeat))
            environment = {
                "CDT_HEARTBEAT_FILE": str(heartbeat),
                "CDT_CHECK_INTERVAL_SECONDS": "60",
            }
            with mock.patch.dict(os.environ, environment, clear=True):
                self.assertEqual(0, cdt_guard.healthcheck())


if __name__ == "__main__":
    unittest.main()
