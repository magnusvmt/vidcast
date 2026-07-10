def test_root_reports_service_identity(client):
    response = client.get("/")

    assert response.status_code == 200
    assert response.json()["service"] == "users"


def test_healthz_checks_db_connectivity(client):
    response = client.get("/healthz")

    assert response.status_code == 200
    assert response.json() == {"status": "ok"}
