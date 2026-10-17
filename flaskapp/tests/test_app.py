import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import ANY, patch

from flaskapp.app import available_polar_options, create_app


class FlaskInterfaceTests(unittest.TestCase):
    def setUp(self):
        self.client = create_app("http://go-api.test").test_client()

    def test_index_is_available(self):
        response = self.client.get("/")

        self.assertEqual(response.status_code, 200)
        page = response.get_data(as_text=True)
        self.assertIn('id="map"', page)
        self.assertIn('id="calculate-route"', page)
        self.assertIn('id="route-form-panel"', page)
        self.assertIn('id="show-route-details"', page)
        self.assertIn('id="route-details-panel"', page)
        self.assertIn('id="route-start"', page)
        self.assertIn('id="route-checkpoints-list"', page)
        self.assertIn('checkpoint?.onLand !== true', page)
        self.assertIn('id="route-polar"', page)
        self.assertIn('value="../data/polars/super_maxi_100.json"', page)
        self.assertIn('id: "vr-checkpoint-lines"', page)
        self.assertIn('id: "vr-checkpoint-rhumb-lines"', page)
        self.assertIn('id: "vr-checkpoint-marks"', page)
        self.assertIn('id: "vr-checkpoint-mark-labels"', page)
        self.assertIn('id: "vr-checkpoint-labels"', page)
        self.assertIn('value="../data/polars/ultim_bp_11.json"', page)
        self.assertIn("/static/route_stream.js", page)
        self.assertIn("/static/route_details.js", page)
        self.assertIn("/api/practical-headings/stream", page)

    def test_polar_options_are_discovered_from_the_data_directory(self):
        with TemporaryDirectory() as temporary_directory:
            polar_directory = Path(temporary_directory)
            (polar_directory / "Zeta.json").write_text("{}", encoding="utf-8")
            (polar_directory / "alpha.json").write_text("{}", encoding="utf-8")
            (polar_directory / "not-a-polar.txt").write_text("{}", encoding="utf-8")

            with patch("flaskapp.app.POLARS_DIR", polar_directory):
                self.assertEqual(
                    available_polar_options(),
                    [
                        {"label": "alpha.json", "value": "../data/polars/alpha.json"},
                        {"label": "Zeta.json", "value": "../data/polars/Zeta.json"},
                    ],
                )

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

    def test_weather_refresh_does_not_change_departure_time(self):
        response = self.client.get("/")

        self.assertEqual(response.status_code, 200)
        page = response.get_data(as_text=True)
        self.assertNotIn('route-instant").value = runToLocal(latestWeatherRun)', page)

    def test_vr_state_polling_does_not_clear_the_route(self):
        response = self.client.get("/")

        self.assertEqual(response.status_code, 200)
        page = response.get_data(as_text=True)
        polling_function = page.split("async function refreshVirtualRegattaState", 1)[1].split(
            "function updateMapFromForm", 1
        )[0]
        self.assertNotIn("updateMapFromForm()", polling_function)

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
