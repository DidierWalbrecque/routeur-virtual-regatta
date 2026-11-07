import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import ANY, patch

from flaskapp.app import create_app


class FlaskRelayTests(unittest.TestCase):
    def setUp(self):
        self.client = create_app("http://go-api.test").test_client()

    def test_index_serves_the_built_react_interface(self):
        with TemporaryDirectory() as temporary_directory:
            dist = Path(temporary_directory)
            (dist / "assets").mkdir()
            (dist / "index.html").write_text('<div id="root"></div>', encoding="utf-8")
            (dist / "assets" / "app.js").write_text("console.log(1)", encoding="utf-8")

            with patch("flaskapp.app.FRONTEND_DIST", dist):
                page = self.client.get("/")
                asset = self.client.get("/assets/app.js")
                missing = self.client.get("/assets/absent.js")
                page_text = page.get_data(as_text=True)
                for response in (page, asset, missing):
                    response.close()

            self.assertEqual(page.status_code, 200)
            self.assertIn('id="root"', page_text)
            self.assertEqual(asset.status_code, 200)
            self.assertEqual(missing.status_code, 404)

    def test_index_explains_how_to_build_the_interface(self):
        with TemporaryDirectory() as temporary_directory:
            with patch("flaskapp.app.FRONTEND_DIST", Path(temporary_directory)):
                response = self.client.get("/")
                assets = self.client.get("/assets/app.js")
                assets.close()

        self.assertEqual(response.status_code, 503)
        self.assertIn("npm run build", response.get_data(as_text=True))
        self.assertEqual(assets.status_code, 404)

    @patch("flaskapp.app._call_go_get_api")
    def test_polars_list_is_proxied(self, call_go_get_api):
        call_go_get_api.return_value = (
            {"polars": [{"label": "a.json", "value": "../data/polars/a.json"}], "default": "../data/polars/a.json"},
            200,
        )

        response = self.client.get("/api/polars")

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.get_json()["default"], "../data/polars/a.json")
        call_go_get_api.assert_called_once_with(ANY, "/api/polars")

    @patch("flaskapp.app._call_go_api")
    def test_weather_refresh_is_proxied(self, call_go_api):
        call_go_api.return_value = ({"run": "2026092112", "files": []}, 200)

        response = self.client.post(
            "/api/weather/refresh",
            json={"forecastHours": [0, 3], "lookbackHours": 48},
        )

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.get_json()["run"], "2026092112")
        call_go_api.assert_called_once_with(
            ANY,
            "/api/weather/refresh",
            {"forecastHours": [0, 3], "lookbackHours": 48},
        )

    @patch("flaskapp.app._call_go_get_api")
    def test_weather_current_and_status_are_proxied(self, call_go_get_api):
        for path in ("/api/weather/current", "/api/weather/status"):
            with self.subTest(path=path):
                call_go_get_api.reset_mock()
                call_go_get_api.return_value = (
                    {"run": "2026110712", "files": ["gfs.2026110712.f000.grb2"]},
                    200,
                )

                response = self.client.get(path)

                self.assertEqual(response.status_code, 200)
                self.assertEqual(response.get_json()["run"], "2026110712")
                call_go_get_api.assert_called_once_with(ANY, path)

    @patch("flaskapp.app._call_go_api")
    def test_vr_state_post_is_proxied(self, call_go_api):
        call_go_api.return_value = ({"stamina": 87.5}, 200)
        payload = {
            "position": {"latitude": 49.8439, "longitude": -5.2075},
            "stamina": 87.5,
            "options": ["heavy"],
        }

        response = self.client.post("/api/vr/state", json=payload)

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.get_json()["stamina"], 87.5)
        call_go_api.assert_called_once_with(ANY, "/api/vr/state", payload)

    @patch("flaskapp.app._call_go_get_api")
    def test_vr_state_get_is_proxied(self, call_go_get_api):
        call_go_get_api.return_value = ({"stamina": 87.5}, 200)

        response = self.client.get("/api/vr/state")

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.get_json()["stamina"], 87.5)
        call_go_get_api.assert_called_once_with(ANY, "/api/vr/state")

    @patch("flaskapp.app._call_go_api")
    def test_calculation_is_proxied(self, call_go_api):
        call_go_api.return_value = (
            {"destinationReached": False, "route": [], "performance": {}},
            200,
        )
        payload = {
            "weatherFiles": ["weather.grb2"],
            "polarPath": "polar.json",
            "position": {"virtualRegatta": "start"},
            "destination": {"virtualRegatta": "destination"},
        }

        response = self.client.post("/api/practical-headings", json=payload)

        self.assertEqual(response.status_code, 200)
        self.assertFalse(response.get_json()["destinationReached"])
        call_go_api.assert_called_once_with(ANY, "/api/practical-headings", payload)

    @patch("flaskapp.app.urllib.request.urlopen")
    def test_calculation_stream_is_relayed_without_decoding(self, urlopen):
        class FakeUpstream:
            status = 200

            def __init__(self):
                self.chunks = [
                    b"event: start\ndata: {\"totalLayers\":2}\n\n",
                    b"event: frontier\ndata: {\"completedLayers\":1}\n\n",
                    b"event: complete\ndata: {\"destinationReached\":true}\n\n",
                ]
                self.closed = False

            def read(self, _size):
                return self.chunks.pop(0) if self.chunks else b""

            def close(self):
                self.closed = True

        upstream = FakeUpstream()
        urlopen.return_value = upstream
        response = self.client.post(
            "/api/practical-headings/stream",
            json={"durationMinutes": 20},
        )

        self.assertEqual(response.status_code, 200)
        self.assertEqual(response.content_type, "text/event-stream")
        self.assertEqual(
            response.get_data(),
            b"".join(
                [
                    b"event: start\ndata: {\"totalLayers\":2}\n\n",
                    b"event: frontier\ndata: {\"completedLayers\":1}\n\n",
                    b"event: complete\ndata: {\"destinationReached\":true}\n\n",
                ]
            ),
        )
        self.assertTrue(upstream.closed)
        request = urlopen.call_args.args[0]
        self.assertEqual(request.full_url, "http://go-api.test/api/practical-headings/stream")
        self.assertEqual(request.get_header("Accept"), "text/event-stream")


if __name__ == "__main__":
    unittest.main()
