import os
import random
import tarfile
import unittest
import urllib

import requests

from .fixtures import APITestCase


def render_test_file(file_name, size, mime):
    random_bytes = random.randbytes(size - len(mime))

    file_data = bytearray(mime)
    file_data.extend(random_bytes)

    with open(file_name, "wb") as f:
        f.write(file_data)


class ArtifactTestCase(APITestCase):
    artifact_name = "quay.io/myimage/myartifact:latest"
    artifact_type = "application/testType"
    remote_artifact_name = "quay.io/libpod/testartifact:20250206-single"
    test_files = [
        {"filename": "test_file_1", "size": 1048576},
        {"filename": "test_file_2", "size": 524288},
    ]

    def test_add(self):
        png_file_sig = bytes([137, 80, 78, 71, 13, 10, 26, 10])

        file_name = self.test_files[0].get("filename")
        file_size = self.test_files[0].get("size")

        # Create test file imitating a png image
        render_test_file(file_name, file_size, png_file_sig)

        parameters = {
            "Name": self.artifact_name,
            "fileName": file_name,
            # FIX: "annotations": '["test:true", "test1:false"]',
            "artifactMIMEType": self.artifact_type,
        }
        with open(file_name, "rb") as file_to_upload:
            file_content = file_to_upload.read()
            r = requests.post(
                self.uri("/artifacts/add"),
                data=file_content,
                params=parameters,
            )

        os.remove(file_name)

        self.assertEqual(r.status_code, 201, r.text)

        # Response looks like:
        # {'ArtifactDigest': 'sha256:65626d4c06f9b83567f924b4839056dd5821b1072ee5ed4bf156db404c564ec4'}
        artifact = r.json()
        self.assertIn("sha256:", artifact["ArtifactDigest"])

    def test_add_append(self):
        file_name = self.test_files[1].get("filename")
        file_size = self.test_files[1].get("size")
        with open(file_name, "wb") as f:
            f.write(os.urandom(file_size))

        parameters = {
            "name": self.artifact_name,
            "fileName": file_name,
            "append": "true",
        }
        with open(file_name, "rb") as file_to_upload:
            file_content = file_to_upload.read()
            r = requests.post(
                self.uri("/artifacts/add"),
                data=file_content,
                params=parameters,
            )
        os.remove(file_name)

        self.assertEqual(r.status_code, 201, r.text)

        # return looks like:
        # {'ArtifactDigest': 'sha256:65626d4c06f9b83567f924b4839056dd5821b1072ee5ed4bf156db404c564ec4'}
        artifact = r.json()
        self.assertIn("sha256:", artifact["ArtifactDigest"])

    def test_add_append_type(self):
        file = "/bin/cat"
        parameters = {
            "name": self.artifact_name,
            "fileName": "cat",
            "artifactMIMEType": "application/octet-stream",
            "append": "true",
        }
        with open(file, "rb") as file_to_upload:
            file_content = file_to_upload.read()
            r = requests.post(
                self.uri("/artifacts/add"),
                data=file_content,
                params=parameters,
            )

        self.assertEqual(r.status_code, 500, r.text)

    def test_add_bad_param(self):
        parameters = {
            "fake": "fake",
        }
        r = requests.post(self.uri("/artifacts/add"), params=parameters)

        self.assertEqual(r.status_code, 400, r.text)

    # FIX: def test_add_empty_file(self):

    def test_inspect(self):
        url = self.uri(
            "/artifacts/" + urllib.parse.quote(self.artifact_name, safe="") + "/json",
        )
        r = requests.get(url)

        self.assertEqual(r.status_code, 200, r.text)

        data = r.json()
        expected_top_level = {"Manifest", "Name", "Digest"}
        expected_manifest = {
            "schemaVersion",
            "mediaType",
            "artifactType",
            "config",
            "layers",
        }
        expected_config = {"mediaType", "digest", "size", "data"}
        expected_layer = {"mediaType", "digest", "size", "annotations"}

        missing_top = expected_top_level - data.keys()
        manifest = data.get("Manifest", {})
        missing_manifest = expected_manifest - manifest.keys()
        config = manifest.get("config", {})
        missing_config = expected_config - config.keys()

        layers = manifest.get("layers", [])
        for i, layer in enumerate(layers):
            missing_layer = expected_layer - layer.keys()
            self.assertFalse(missing_layer)

        # Assert blob media type detection is working
        self.assertEqual(layers[0]["mediaType"], "image/png")

        # Assert blob media type fallback detection is working
        self.assertEqual(layers[1]["mediaType"], "application/octet-stream")

        # Assert added blob is the correct size
        self.assertEqual(layers[0]["size"], self.test_files[0].get("size"))
        self.assertEqual(layers[1]["size"], self.test_files[1].get("size"))

        # Assert artifactType is set correctly
        self.assertEqual(data["Manifest"]["artifactType"], self.artifact_type)
        self.assertEqual(data["Manifest"]["artifactType"], self.artifact_type)

        # Assert all missing dicts are empty
        self.assertFalse(missing_top)
        self.assertFalse(missing_manifest)
        self.assertFalse(missing_config)

    def test_inspect_absent_artifact(self):
        url = self.uri("/artifacts/" + "fake_artifact" + "/json")
        r = requests.get(url)

        self.assertEqual(r.status_code, 404, r.text)

    def test_list(self):
        url = self.uri("/artifacts/json")
        r = requests.get(url)

        self.assertEqual(r.status_code, 200, r.text)

        returned_json = r.json()

        expected_top_level = {"Manifest", "Name"}
        expected_manifest = {"schemaVersion", "mediaType", "config", "layers"}
        expected_config = {"mediaType", "digest", "size", "data"}
        expected_layer = {"mediaType", "digest", "size", "annotations"}

        for data in returned_json:
            missing_top = expected_top_level - data.keys()
            manifest = data.get("Manifest", {})
            missing_manifest = expected_manifest - manifest.keys()
            config = manifest.get("config", {})
            missing_config = expected_config - config.keys()

            layers = manifest.get("layers", [])
            for i, layer in enumerate(layers):
                missing_layer = expected_layer - layer.keys()
                self.assertFalse(missing_layer)

            # assert all missing dicts are empty
            self.assertFalse(missing_top)
            self.assertFalse(missing_manifest)
            self.assertFalse(missing_config)

    def test_pull(self):
        url = self.uri("/artifacts/pull")
        parameters = {
            "name": self.remote_artifact_name,
        }
        r = requests.post(url, params=parameters)

        self.assertEqual(r.status_code, 200, r.text)

    def test_pull_retry(self):
        # Note: Default retry is 3 attempts with 1s delay.
        url = self.uri("/artifacts/pull")
        parameters = {
            "name": "localhost/fake/artifact:latest",
            "retryDelay": "3s",
            "retry": "2",
        }
        r = requests.post(url, params=parameters)

        self.assertEqual(r.status_code, 500, r.text)
        self.assertAlmostEqual(r.elapsed.total_seconds(), 6.0, places=1)

    def test_remove(self):
        url = self.uri(
            "/artifacts/" + urllib.parse.quote(self.remote_artifact_name, safe=""),
        )
        r = requests.delete(url)

        self.assertEqual(r.status_code, 200, r.text)

        artifact = r.json()
        self.assertIn("sha256:", artifact["ArtifactDigests"][0])

    def test_remove_absent_artifact(self):
        url = self.uri(
            "/artifacts/"
            + urllib.parse.quote("localhost/fake/artifact:latest", safe=""),
        )
        r = requests.delete(url)

        self.assertEqual(r.status_code, 404, r.text)

    def test_push(self):
        url = self.uri(
            "/artifacts/" + urllib.parse.quote(self.artifact_name, safe="") + "/push",
        )
        r = requests.post(url)

        self.assertEqual(r.status_code, 500, r.text)

    def test_push_bad_param(self):
        parameters = {
            "retry": "abc",
        }
        url = self.uri(
            "/artifacts/" + urllib.parse.quote(self.artifact_name, safe="") + "/push",
        )
        r = requests.post(
            url,
            params=parameters,
        )

        self.assertEqual(r.status_code, 400, r.text)

    # FIX: Not sure if I should really push an artifact from this test

    def test_extract(self):
        tar_file = "test.tar"
        url = self.uri(
            "/artifacts/"
            + urllib.parse.quote(self.artifact_name, safe="")
            + "/extract",
        )
        r = requests.get(url)

        with open(tar_file, "wb") as f:
            f.write(r.content)

        all_match = True
        with tarfile.open(tar_file, "r") as tar:
            for member in tar.getmembers():
                tar_file_sizes = {
                    member.name: member.size
                    for member in tar.getmembers()
                    if member.isfile()
                }
            for expected_file in self.test_files:
                filename = expected_file.get("filename")
                expected_size = expected_file.get("size")

                if filename not in tar_file_sizes:
                    all_match = False
                else:
                    actual_size = tar_file_sizes[filename]
                    if actual_size != expected_size:
                        all_match = False

        os.remove(tar_file)

        self.assertTrue(all_match)
        self.assertEqual(r.status_code, 200, r.text)

    # FIX:
    # def test_extract_digest(self):


if __name__ == "__main__":
    unittest.main()
