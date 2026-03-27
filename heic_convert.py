"""Convert HEIC image data from stdin to PNG on stdout."""
import sys
from PIL import Image
import pillow_heif

pillow_heif.register_heif_opener()

img = Image.open(sys.stdin.buffer)
img.save(sys.stdout.buffer, format="PNG")
