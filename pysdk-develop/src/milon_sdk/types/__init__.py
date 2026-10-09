from .bitbap import Bitmap64, BITS, low_bits_mask


def new_bitmap64(raw: int = 0) -> Bitmap64:
    """Go: NewBitmap64."""
    return Bitmap64(raw)


__all__ = ["Bitmap64", "BITS", "low_bits_mask", "new_bitmap64"]
