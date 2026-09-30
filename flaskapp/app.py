"""Interface web Flask qui délègue les calculs à l'API Go."""

from __future__ import annotations

import json
import logging
import os
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any

from flask import (
    Flask,
    Response,
    jsonify,
    render_template,
    request,
    send_from_directory,
    stream_with_context,
)


DEFAULT_API_BASE_URL = "http://localhost:8080"
DEFAULT_TIMEOUT_SECONDS = 300
MAPLIBRE_DIST = Path(__file__).resolve().parents[1] / "frontend" / "node_modules" / "maplibre-gl" / "dist"
POLARS_DIR = Path(__file__).resolve().parents[1] / "data" / "polars"
DEFAULT_POLAR_PATH = "../data/polars/ultim_bp_11.json"


def available_polar_options() -> list[dict[str, str]]:
    """Retourne les polaires JSON disponibles dans le répertoire du projet."""

    try:
        polar_files = sorted(POLARS_DIR.glob("*.json"), key=lambda path: path.name.lower())
    except OSError:
        return []

    return [
        {
            "label": polar_file.name,
            "value": f"../data/polars/{polar_file.name}",
        }
        for polar_file in polar_files
        if polar_file.is_file()
    ]


def create_app(api_base_url: str | None = None) -> Flask:
    """Crée l'application Flask et ses routes HTTP."""

    app = Flask(__name__)
    app.logger.setLevel(logging.INFO)
    app.config["API_BASE_URL"] = (
        api_base_url or os.environ.get("ROUTEUR_API_URL", DEFAULT_API_BASE_URL)
    ).rstrip("/")
    app.config["API_TIMEOUT_SECONDS"] = int(
        os.environ.get("ROUTEUR_API_TIMEOUT_SECONDS", DEFAULT_TIMEOUT_SECONDS)
    )

    @app.get("/")
    def index():
        polar_options = available_polar_options()
        default_polar = next(
            (
                option["value"]
                for option in polar_options
                if option["value"] == DEFAULT_POLAR_PATH
            ),
            polar_options[0]["value"] if polar_options else DEFAULT_POLAR_PATH,
        )
        return render_template(
            "index.html",
            polar_options=polar_options,
            default_polar=default_polar,
        )

    @app.get("/vendor/maplibre/<path:filename>")
    def maplibre_vendor(filename):
        return send_from_directory(str(MAPLIBRE_DIST), filename)

    @app.post("/api/weather/refresh")
    def refresh_weather():
        payload = request.get_json(silent=True) or {}
        app.logger.info("[GRIB] Demande d’actualisation envoyée")
        app.logger.info("[GRIB] Actualisation demandée : %s", payload)
        body, status = _call_go_api(app, "/api/weather/refresh", payload)
        app.logger.info("[GRIB] Réponse reçue : HTTP %s", status)
        if status < 400 and isinstance(body, dict):
            files = body.get("files") or []
            removed_files = body.get("removedFiles") or []
            app.logger.info(
                "[GRIB] Actualisation réussie : run %s, %d fichier(s), %d ancien(s) supprimé(s)",
                body.get("run", "inconnu"),
                len(files),
                len(removed_files),
            )
        return jsonify(body), status

    @app.post("/api/practical-headings")
    def calculate_practical_headings():
        payload = request.get_json(silent=True) or {}
        app.logger.info("[ROUTE] Demande de calcul reçue")
        app.logger.info(
            "[ROUTE] État Virtual Regatta utilisé : %s",
            payload.get("useVirtualRegattaState", False),
        )
        body, status = _call_go_api(app, "/api/practical-headings", payload)
        app.logger.info("[ROUTE] Réponse reçue : HTTP %s", status)
        if status < 400 and isinstance(body, dict):
            app.logger.info(
                "[ROUTE] Calcul terminé : %d point(s), %d front(s) d’isochrone, destination atteinte=%s",
                len(body.get("route") or []),
                len(body.get("isochrones") or []),
                body.get("destinationReached", False),
            )
        return jsonify(body), status

    @app.post("/api/practical-headings/stream")
    def stream_practical_headings():
        payload = request.get_json(silent=True) or {}
        app.logger.info("[ROUTE] Demande de calcul streaming reçue")
        upstream, body, status = _open_go_stream(
            app, "/api/practical-headings/stream", payload
        )
        if upstream is None:
            return jsonify(body), status

        return Response(
            stream_with_context(_relay_go_stream(upstream)),
            status=status,
            headers={
                "Content-Type": "text/event-stream",
                "Cache-Control": "no-cache",
                "X-Accel-Buffering": "no",
            },
        )

    @app.post("/api/vr/state")
    def receive_vr_state():
        payload = request.get_json(silent=True) or {}
        app.logger.info(
            "[VR STATE] Réception : user=%s, course=%s.%s, position=%s, stamina=%s, options=%s, voiles=%s, portes=%d",
            payload.get("userId", ""),
            payload.get("raceId", ""),
            payload.get("legNum", ""),
            payload.get("position", {}),
            payload.get("stamina"),
            payload.get("options", []),
            payload.get("sails", []),
            len(payload.get("checkpoints") or []),
        )
        body, status = _call_go_api(app, "/api/vr/state", payload)
        app.logger.info("[VR STATE] Réponse Go : HTTP %s", status)
        if status >= 400:
            app.logger.error("[VR STATE] Erreur Go : %s", body)
        return jsonify(body), status

    @app.get("/api/vr/state")
    def read_vr_state():
        app.logger.info("[VR STATE] Lecture du dernier état demandé")
        body, status = _call_go_get_api(app, "/api/vr/state")
        app.logger.info("[VR STATE] Réponse Go : HTTP %s", status)
        if status >= 400:
            app.logger.error("[VR STATE] Erreur Go : %s", body)
        return jsonify(body), status

    return app


def _call_go_api(app: Flask, path: str, payload: dict[str, Any]) -> tuple[Any, int]:
    """Transmet une requête JSON à l'API Go et conserve son statut HTTP."""

    url = f"{app.config['API_BASE_URL']}{path}"
    body = json.dumps(payload).encode("utf-8")
    http_request = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(
            http_request, timeout=app.config["API_TIMEOUT_SECONDS"]
        ) as response:
            return _decode_json(response.read()), response.status
    except urllib.error.HTTPError as error:
        app.logger.error("[API Go] Réponse HTTP %s pour %s", error.code, path)
        return _decode_json(error.read()), error.code
    except (urllib.error.URLError, TimeoutError, OSError) as error:
        app.logger.error("[API Go] Appel impossible vers %s : %s", path, error)
        return {"error": f"API Go indisponible : {error}"}, 502


def _call_go_get_api(app: Flask, path: str) -> tuple[Any, int]:
    """Transmet une requête GET à l'API Go et conserve son statut HTTP."""

    url = f"{app.config['API_BASE_URL']}{path}"
    http_request = urllib.request.Request(url, method="GET")

    try:
        with urllib.request.urlopen(
            http_request, timeout=app.config["API_TIMEOUT_SECONDS"]
        ) as response:
            return _decode_json(response.read()), response.status
    except urllib.error.HTTPError as error:
        app.logger.error("[API Go] Réponse HTTP %s pour %s", error.code, path)
        return _decode_json(error.read()), error.code
    except (urllib.error.URLError, TimeoutError, OSError) as error:
        app.logger.error("[API Go] Appel impossible vers %s : %s", path, error)
        return {"error": f"API Go indisponible : {error}"}, 502


def _open_go_stream(
    app: Flask, path: str, payload: dict[str, Any]
) -> tuple[Any | None, Any, int]:
    """Ouvre le flux Go et conserve la réponse ouverte pour le générateur Flask."""

    url = f"{app.config['API_BASE_URL']}{path}"
    body = json.dumps(payload).encode("utf-8")
    http_request = urllib.request.Request(
        url,
        data=body,
        headers={"Content-Type": "application/json", "Accept": "text/event-stream"},
        method="POST",
    )

    try:
        upstream = urllib.request.urlopen(
            http_request, timeout=app.config["API_TIMEOUT_SECONDS"]
        )
        return upstream, None, upstream.status
    except urllib.error.HTTPError as error:
        app.logger.error("[API Go] Réponse HTTP %s pour %s", error.code, path)
        return None, _decode_json(error.read()), error.code
    except (urllib.error.URLError, TimeoutError, OSError) as error:
        app.logger.error("[API Go] Flux impossible vers %s : %s", path, error)
        return None, {"error": f"API Go indisponible : {error}"}, 502


def _relay_go_stream(upstream: Any):
    """Relaye les octets SSE au navigateur sans attendre la fin du calcul Go."""

    try:
        while True:
            chunk = upstream.read(16 * 1024)
            if not chunk:
                break
            yield chunk
    finally:
        upstream.close()


def _decode_json(raw_body: bytes) -> Any:
    """Décode la réponse Go, même si elle n'est pas un JSON valide."""

    try:
        return json.loads(raw_body.decode("utf-8"))
    except (UnicodeDecodeError, json.JSONDecodeError):
        return {"error": raw_body.decode("utf-8", errors="replace")}


if __name__ == "__main__":
    create_app().run(
        host="127.0.0.1",
        port=int(os.environ.get("FLASK_PORT", "5000")),
        debug=True,
    )
