#!/usr/bin/env python3
"""Unit tests for bump_image_tag.py.

Run with: cd .github/scripts && python3 -m unittest test_bump_image_tag.py -v
"""
import unittest

from bump_image_tag import bump_tag


class BumpTagTopLevelImageBlock(unittest.TestCase):
    """Shape of deploy/charts/*/values.yaml: image: at column 0."""

    def setUp(self):
        self.text = (
            "replicaCount: 2\n"
            "\n"
            "image:\n"
            "  # a comment explaining the repository choice\n"
            "  repository: vidcast-registry:5000/echo\n"
            "  tag: dev\n"
            "  pullPolicy: IfNotPresent\n"
            "\n"
            "service:\n"
            "  port: 80\n"
        )

    def test_replaces_tag_value(self):
        result = bump_tag(self.text, "abc123")
        self.assertIn("  tag: abc123\n", result)
        self.assertNotIn("tag: dev", result)

    def test_preserves_everything_else(self):
        result = bump_tag(self.text, "abc123")
        expected = self.text.replace("tag: dev", "tag: abc123")
        self.assertEqual(result, expected)

    def test_does_not_touch_repository(self):
        result = bump_tag(self.text, "abc123")
        self.assertIn("  repository: vidcast-registry:5000/echo\n", result)


class BumpTagNestedImageBlock(unittest.TestCase):
    """Shape of deploy/argocd/apps/*.yaml: image: nested under helm.valuesObject."""

    def setUp(self):
        self.text = (
            "apiVersion: argoproj.io/v1alpha1\n"
            "kind: Application\n"
            "spec:\n"
            "  source:\n"
            "    path: deploy/charts/echo\n"
            "    helm:\n"
            "      valuesObject:\n"
            "        image:\n"
            "          repository: ghcr.io/magnusvmt/vidcast/echo\n"
            "          tag: latest\n"
            "  destination:\n"
            "    namespace: apps\n"
        )

    def test_replaces_nested_tag_value(self):
        result = bump_tag(self.text, "deadbeef")
        self.assertIn("          tag: deadbeef\n", result)
        self.assertNotIn("tag: latest", result)

    def test_preserves_indentation_and_siblings(self):
        result = bump_tag(self.text, "deadbeef")
        self.assertIn("          repository: ghcr.io/magnusvmt/vidcast/echo\n", result)
        self.assertIn("  destination:\n", result)
        self.assertIn("    namespace: apps\n", result)

    def test_does_not_touch_unrelated_path_field(self):
        # 'path:' is not 'tag:' and lives outside the image: block - regression
        # guard against overly broad matching.
        result = bump_tag(self.text, "deadbeef")
        self.assertIn("    path: deploy/charts/echo\n", result)


class BumpTagQuotedValue(unittest.TestCase):
    def test_replaces_quoted_tag(self):
        text = "image:\n  repository: bluenviron/mediamtx\n  tag: \"1.19.2\"\n"
        result = bump_tag(text, "1.20.0")
        self.assertIn('  tag: 1.20.0\n', result)


class BumpTagMissingField(unittest.TestCase):
    def test_raises_when_no_image_block(self):
        with self.assertRaises(ValueError):
            bump_tag("service:\n  port: 80\n", "abc123")

    def test_raises_when_image_block_has_no_tag(self):
        text = "image:\n  repository: foo\n  pullPolicy: IfNotPresent\nservice:\n  port: 80\n"
        with self.assertRaises(ValueError):
            bump_tag(text, "abc123")


if __name__ == "__main__":
    unittest.main()
