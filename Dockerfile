# syntax=docker/dockerfile:1

FROM python:3.12-slim

ARG VERSION=dev

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    CDT_GUARD_VERSION=${VERSION} \
    CDT_CHECK_INTERVAL_SECONDS=60 \
    CDT_CONTROL_MODE=keep_running \
    CDT_TRAFFIC_THRESHOLD_GB=180 \
    ECS_STOPPED_MODE=StopCharging \
    ECS_FORCE_STOP=false \
    CDT_HEARTBEAT_FILE=/tmp/cdt-guard-heartbeat

WORKDIR /app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY scripts/cdt_guard.py ./cdt_guard.py

RUN useradd --create-home --shell /usr/sbin/nologin app
USER app

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD python /app/cdt_guard.py --healthcheck

ENTRYPOINT ["python", "/app/cdt_guard.py"]
