import unittest
from unittest.mock import ANY, patch

from flaskapp.app import create_app


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
        self.assertIn('id="route-start"', page)

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


if __name__ == "__main__":
    unittest.main()
