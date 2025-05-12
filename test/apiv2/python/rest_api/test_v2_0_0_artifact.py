import os
import tarfile
import unittest
from typing import cast

import requests

from .fixtures import APITestCase
from .fixtures.api_testcase import Artifact, ArtifactFile


class ArtifactTestCase(APITestCase):
    def test_add(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact:latest"
        file = ArtifactFile()
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
        }

        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        add_response = artifact.add()

        # Assert correct response code
        self.assertEqual(add_response.status_code, 201, add_response.text)

        # Assert return response is json and contains digest
        add_response_json = add_response.json()
        self.assertIn("sha256:", cast(str, add_response_json["ArtifactDigest"]))

        inspect_response_json = artifact.do_artifact_inspect_request().json()
        artifact_layer = inspect_response_json["Manifest"]["layers"][0]

        # Assert uploaded artifact blob is expected size
        self.assertEqual(artifact_layer["size"], file.size)

        # Assert uploaded artifact blob has expected title annotation
        self.assertEqual(
            artifact_layer["annotations"]["org.opencontainers.image.title"], file.name
        )

        # Assert blob media type fallback detection is working
        self.assertEqual(artifact_layer["mediaType"], "application/octet-stream")

    def test_add_with_append(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact:latest"
        file = ArtifactFile(name="test_file_2")
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
            "append": "true",
        }
        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        add_response = artifact.add()

        # Assert correct response code
        self.assertEqual(add_response.status_code, 201, add_response.text)

        # Assert return response is json and contains digest
        add_response_json = add_response.json()
        self.assertIn("sha256:", cast(str, add_response_json["ArtifactDigest"]))

        inspect_response_json = artifact.do_artifact_inspect_request().json()
        artifact_layers = inspect_response_json["Manifest"]["layers"]

        # Assert artifact now has two layers
        self.assertEqual(len(artifact_layers), 2)

    def test_add_with_artifactMIMEType_override(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact_artifactType:latest"
        file = ArtifactFile()
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
            "artifactMIMEType": "application/testType",
        }

        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        add_response = artifact.add()

        # Assert correct response code
        self.assertEqual(add_response.status_code, 201, add_response.text)

        inspect_response_json = artifact.do_artifact_inspect_request().json()

        # Assert added artifact has correct mediaType
        self.assertEqual(
            inspect_response_json["Manifest"]["artifactType"], "application/testType"
        )

    def test_add_with_annotations(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact_annotation:latest"
        file = ArtifactFile()
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
            "annotations": ["test=test", "foo=bar"],
        }

        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        add_response = artifact.add()

        # Assert correct response code
        self.assertEqual(add_response.status_code, 201, add_response.text)

        inspect_response_json = artifact.do_artifact_inspect_request().json()
        artifact_layer = inspect_response_json["Manifest"]["layers"][0]

        # Assert artifactBlobAnnotation is set correctly
        anno = {
            "foo": "bar",
            "org.opencontainers.image.title": artifact.file.name,
            "test": "test",
        }
        self.assertEqual(artifact_layer["annotations"], anno)

    def test_add_with_empty_file(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact_empty_file:latest"
        file = ArtifactFile(size=0)
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
        }
        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        add_response = artifact.add()

        # Assert correct response code
        self.assertEqual(add_response.status_code, 201, add_response.text)

        # Assert return response is json and contains digest
        add_response_json = add_response.json()
        self.assertIn("sha256:", cast(str, add_response_json["ArtifactDigest"]))

        inspect_response_json = artifact.do_artifact_inspect_request().json()
        artifact_layer = inspect_response_json["Manifest"]["layers"][0]

        # Assert uploaded artifact blob is expected size
        self.assertEqual(artifact_layer["size"], file.size)

        # Assert uploaded artifact blob has expected title annotation
        self.assertEqual(
            artifact_layer["annotations"]["org.opencontainers.image.title"], file.name
        )

    def test_add_with_fileMIMEType_override(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact_mime_type:latest"
        file = ArtifactFile()
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
            "fileMIMEType": "fake/type",
        }
        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        add_response = artifact.add()

        # Assert correct response code
        self.assertEqual(add_response.status_code, 201, add_response.text)

        # Assert return response is json and contains digest
        add_response_json = add_response.json()
        self.assertIn("sha256:", cast(str, add_response_json["ArtifactDigest"]))

        inspect_response_json = artifact.do_artifact_inspect_request().json()
        artifact_layer = inspect_response_json["Manifest"]["layers"][0]

        # Assert uploaded artifact blob is expected MIME type
        self.assertEqual(artifact_layer["mediaType"], "fake/type")

    def test_add_with_auto_fileMIMEType_discovery(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact_image_blob:latest"
        FILE_SIG = bytes([137, 80, 78, 71, 13, 10, 26, 10])
        file = ArtifactFile(sig=FILE_SIG)
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
        }

        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        add_response = artifact.add()

        # Assert correct response code
        self.assertEqual(add_response.status_code, 201, add_response.text)

        # Assert return response is json and contains digest
        add_response_json = add_response.json()
        self.assertIn("sha256:", cast(str, add_response_json["ArtifactDigest"]))

        inspect_response_json = artifact.do_artifact_inspect_request().json()
        artifact_layer = inspect_response_json["Manifest"]["layers"][0]

        # Assert uploaded artifact blob is automatically recognised as image
        self.assertEqual(artifact_layer["mediaType"], "image/png")

    def test_add_append_with_type_fails(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact:latest"
        file = ArtifactFile()
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
            "artifactMIMEType": "application/octet-stream",
            "append": "true",
        }
        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        r = artifact.add()
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 500, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "append option is not compatible with ArtifactType option",
        )

    def test_add_with_append_to_missing_artifact_fails(self):
        ARTIFACT_NAME = "quay.io/myimage/missing:latest"
        file = ArtifactFile()
        parameters: dict[str, str | list[str]] = {
            "name": ARTIFACT_NAME,
            "fileName": file.name,
            "append": "true",
        }
        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        r = artifact.add()
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 404, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(rjson["cause"], "artifact does not exist")

    def test_add_without_name_and_filename_fails(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact:latest"
        file = ArtifactFile()
        parameters: dict[str, str | list[str]] = {"fake": "fake"}
        artifact = Artifact(self.uri(""), ARTIFACT_NAME, parameters, file)

        r = artifact.add()
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 400, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "name and file parameters are required",
        )

    def test_inspect(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact_mime_type:latest"

        url = self.uri(
            "/artifacts/" + ARTIFACT_NAME + "/json",
        )
        r = requests.get(url)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 200, r.text)

        # Define expected layout keys
        expected_top_level = {"Manifest", "Name", "Digest"}
        expected_manifest = {
            "schemaVersion",
            "mediaType",
            "config",
            "layers",
        }
        expected_config = {"mediaType", "digest", "size", "data"}
        expected_layer = {"mediaType", "digest", "size", "annotations"}

        # Compare returned keys with expected
        missing_top = expected_top_level - rjson.keys()
        manifest = rjson.get("Manifest", {})
        missing_manifest = expected_manifest - manifest.keys()
        config = manifest.get("config", {})
        missing_config = expected_config - config.keys()

        layers = manifest.get("layers", [])
        for i, layer in enumerate(layers):
            missing_layer = expected_layer - layer.keys()
            self.assertFalse(missing_layer)

        # Assert all missing dicts are empty meaning all expected keys were present
        self.assertFalse(missing_top)
        self.assertFalse(missing_manifest)
        self.assertFalse(missing_config)

    def test_inspect_absent_artifact_fails(self):
        ARTIFACT_NAME = "fake_artifact"
        url = self.uri("/artifacts/" + ARTIFACT_NAME + "/json")
        r = requests.get(url)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 404, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "artifact does not exist",
        )

    def test_list(self):
        url = self.uri("/artifacts/json")
        r = requests.get(url)
        rjson = r.json()

        self.assertEqual(r.status_code, 200, r.text)

        expected_top_level = {"Manifest", "Name"}
        expected_manifest = {"schemaVersion", "mediaType", "config", "layers"}
        expected_config = {"mediaType", "digest", "size", "data"}
        expected_layer = {"mediaType", "digest", "size", "annotations"}

        for data in rjson:
            missing_top = expected_top_level - data.keys()
            manifest = data.get("Manifest", {})
            missing_manifest = expected_manifest - manifest.keys()
            config = manifest.get("config", {})
            missing_config = expected_config - config.keys()

            layers = manifest.get("layers", [])
            for _, layer in enumerate(layers):
                missing_layer = expected_layer - layer.keys()
                self.assertFalse(missing_layer)

            # assert all missing dicts are empty
            self.assertFalse(missing_top)
            self.assertFalse(missing_manifest)
            self.assertFalse(missing_config)

    def test_pull(self):
        ARTIFACT_NAME = "quay.io/libpod/testartifact:20250206-single"
        url = self.uri("/artifacts/pull")
        parameters = {
            "name": ARTIFACT_NAME,
        }
        r = requests.post(url, params=parameters)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 200, r.text)

        # Assert return error response is json and contains correct message
        self.assertIn("sha256:", rjson["ArtifactDigest"])

    def test_pull_with_retry(self):
        ARTIFACT_NAME = "localhost/fake/artifact:latest"

        # Note: Default retry is 3 attempts with 1s delay.
        url = self.uri("/artifacts/pull")
        parameters = {
            "name": ARTIFACT_NAME,
            "retryDelay": "3s",
            "retry": "2",
        }
        r = requests.post(url, params=parameters)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 500, r.text)

        # Assert request took expected time with retries
        self.assertTrue(5 < r.elapsed.total_seconds() < 7)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "connection refused",
        )

    def test_pull_unauthorised_fails(self):
        ARTIFACT_NAME = "quay.io/libpod_secret/testartifact:latest"
        url = self.uri("/artifacts/pull")
        parameters = {
            "name": ARTIFACT_NAME,
        }
        r = requests.post(url, params=parameters)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 401, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "unauthorized",
        )

    def test_pull_missing_fails(self):
        ARTIFACT_NAME = "quay.io/libpod/testartifact:superfake"
        url = self.uri("/artifacts/pull")
        parameters = {
            "name": ARTIFACT_NAME,
        }
        r = requests.post(url, params=parameters)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 404, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "manifest unknown",
        )

    def test_remove(self):
        ARTIFACT_NAME = "quay.io/libpod/testartifact:20250206-single"
        url = self.uri("/artifacts/" + ARTIFACT_NAME)
        r = requests.delete(url)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 200, r.text)

        # Assert return response is json and contains digest
        self.assertIn("sha256:", rjson["ArtifactDigests"][0])

    def test_remove_absent_artifact_fails(self):
        ARTIFACT_NAME = "localhost/fake/artifact:latest"
        url = self.uri("/artifacts/" + ARTIFACT_NAME)

        r = requests.delete(url)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 404, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "artifact does not exist",
        )

    def test_push_unauthorised(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact:latest"

        url = self.uri(
            "/artifacts/" + ARTIFACT_NAME + "/push",
        )
        r = requests.post(url)
        rjson = r.json()

        # Assert return error response is json and contains correct message
        self.assertEqual(r.status_code, 401, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "unauthorized",
        )

    def test_push_bad_param(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact:latest"
        parameters = {
            "retry": "abc",
        }
        url = self.uri(
            "/artifacts/" + ARTIFACT_NAME + "/push",
        )
        r = requests.post(
            url,
            params=parameters,
        )
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 400, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "name parameter is required",
        )

    def test_push_missing_artifact(self):
        ARTIFACT_NAME = "localhost/fake/artifact:latest"
        url = self.uri(
            "/artifacts/" + ARTIFACT_NAME + "/push",
        )
        r = requests.post(
            url,
        )
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 404, r.text)

        # Assert return error response is json and contains correct message
        self.assertIn(
            "no descriptor found for reference",
            rjson["cause"],
        )

    def test_extract(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact:latest"

        url = self.uri(
            "/artifacts/" + ARTIFACT_NAME + "/extract",
        )
        r = requests.get(url)

        # Assert correct response code
        self.assertEqual(r.status_code, 200, r.text)

        tar_file = "test.tar"
        tar_file_sizes = None

        with open(tar_file, "wb") as f:
            _ = f.write(r.content)

        with tarfile.open(tar_file, "r") as tar:
            tar_file_sizes = {m.name: m.size for m in tar.getmembers() if m.isfile()}

        self.assertEqual(
            tar_file_sizes, {"test_file_1": 1048576, "test_file_2": 1048576}
        )

        os.remove(tar_file)

    def test_extract_with_title(self):
        ARTIFACT_NAME = "quay.io/myimage/myartifact:latest"

        parameters: dict[str, str] = {
            "title": "test_file_1",
        }
        url = self.uri(
            "/artifacts/" + ARTIFACT_NAME + "/extract",
        )
        r = requests.get(url, parameters)

        # Assert correct response code
        self.assertEqual(r.status_code, 200, r.text)

        tar_file = "test.tar"
        tar_file_sizes = None

        with open(tar_file, "wb") as f:
            _ = f.write(r.content)

        with tarfile.open(tar_file, "r") as tar:
            tar_file_sizes = {m.name: m.size for m in tar.getmembers() if m.isfile()}

        self.assertEqual(tar_file_sizes, {"test_file_1": 1048576})

        os.remove(tar_file)

    def test_extract_absent_fails(self):
        ARTIFACT_NAME = "localhost/fake/artifact:latest"

        url = self.uri(
            "/artifacts/" + ARTIFACT_NAME + "/extract",
        )
        r = requests.get(url)
        rjson = r.json()

        # Assert correct response code
        self.assertEqual(r.status_code, 404, r.text)

        # Assert return error response is json and contains correct message
        self.assertEqual(
            rjson["cause"],
            "artifact does not exist",
        )


if __name__ == "__main__":
    unittest.main()
