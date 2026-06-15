# WIC 解码模块实现文档

> **目标读者**：负责编辑代码的 AI 模型或开发者  
> **状态**：Cargo.toml 已完成，其余 6 个文件待修改  
> **最后更新**：2026-06-15

---

## 1. 总览

将 AVIF 和 JXL 解码统一走 Windows WIC (Windows Imaging Component) 原生 COM 接口，
完全移除现有的 `jxl-oxide` 纯 Rust JXL 解码器。

- 新增可选 Cargo feature `wic`（默认关闭）
- 新增文件 `src/wic.rs`：WIC 解码管线
- 修改 `src/decode.rs`：移除 jxl 分支，添加 wic 分支
- 修改 `src/settings.rs`：扩展列表追加 `.avif` / `.jxl`
- 修改 `src/lib.rs`：条件注册 `mod wic`
- 修改 `.github/workflows/ci.yml`：CI 测试 jxl → wic
- 修改 `README.md`：更新格式说明

---

## 2. 已完成的修改

### 2.1 `Cargo.toml`

**已完成，无需再改。** 变更摘要：

- 删除了 `jxl-oxide = { version = "0.12", features = ["image"], optional = true }` 依赖
- 删除了 `jxl = ["dep:jxl-oxide"]` feature
- 添加了 `wic = []` feature（带注释说明用途）
- 在 `[dependencies].windows.features` 和 `[dev-dependencies].windows.features` 中都添加了 `"Win32_Graphics_Imaging"`

---

## 3. 待创建文件：`src/wic.rs`

### 3.1 设计思路

```
bytes (Vec<u8>)
  → CreateStreamOnHGlobal → IStream
  → CoCreateInstance(CLSID_WICImagingFactory2) → IWICImagingFactory2
  → CreateDecoderFromStream(stream, None, WICDecodeOptions(0)) → IWICBitmapDecoder
  → GetFrame(0) → IWICBitmapFrameDecode
  → 检查尺寸 (limits::MAX_IMAGE_DIMENSION / MAX_IMAGE_ALLOC)
  → IWICFormatConverter::Initialize(frame, GUID_WICPixelFormat32bppRGBA, ...)
  → CopyPixels → Vec<u8>
  → ImageBuffer<Rgba<u8>> → DynamicImage::ImageRgba8
```

### 3.2 完整代码

```rust
//! AVIF / JXL decoding via the Windows Imaging Component (WIC).
//!
//! WIC is the native Windows COM image codec framework.  By delegating
//! to WIC we get AVIF and JPEG XL support with zero new Rust dependencies
//! — the codecs live on the host system (Windows 11 24H2+ ships them
//! built-in; Windows 10 requires the AV1 Image Extensions and/or JPEG
//! XL Extensions from the Microsoft Store).
//!
//! Gated behind the `wic` Cargo feature.  Without it, the module does
//! not exist and the crate compiles with its original format set.

use std::error::Error;
use std::ptr;

use image::{DynamicImage, ImageBuffer, Rgba};
use windows::Win32::Graphics::Imaging::{
    CLSID_WICImagingFactory2, GUID_WICPixelFormat32bppRGBA, IWICBitmapDecoder,
    IWICBitmapFrameDecode, IWICFormatConverter, IWICImagingFactory2, WICBitmapPaletteType,
    WICDecodeOptions, WICPixelFormatTranslate,
};
use windows::Win32::System::Com::{CreateStreamOnHGlobal, IStream};
use windows::core::Interface;

use crate::limits;

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/// Return `true` if the filename (case-insensitive) ends with an
/// extension that should be decoded via WIC — currently `.avif` and
/// `.jxl`.
pub fn is_wic_format(name: &str) -> bool {
    let lower = name.to_ascii_lowercase();
    lower.ends_with(".avif") || lower.ends_with(".jxl")
}

/// Decode arbitrary image bytes through WIC.  Format is auto-detected
/// from magic bytes by the WIC codec, so a mislabelled file will still
/// decode (or fail) correctly.
///
/// Returns a full-resolution `DynamicImage::ImageRgba8`.
///
/// # Errors
///
/// - `WINCODEC_ERR_COMPONENTNOTFOUND` (0x88982F50) → the system has no
///   WIC codec installed for this image format.  Turned into a
///   human-readable message like *"No AVIF codec found"*.
/// - Other WIC HRESULT errors are forwarded with their hex code.
/// - Dimensions exceeding [`limits::MAX_IMAGE_DIMENSION`] or pixel
///   buffer exceeding [`limits::MAX_IMAGE_ALLOC`] cause an error.
pub fn decode_via_wic(bytes: &[u8]) -> Result<DynamicImage, Box<dyn Error>> {
    // ── 1. Wrap bytes in an IStream ────────────────────────────────────
    let stream = create_stream_over_bytes(bytes)?;

    // ── 2. Create the WIC imaging factory ─────────────────────────────
    let factory: IWICImagingFactory2 =
        unsafe { CoCreateInstance(&CLSID_WICImagingFactory2, None, CLSCTX_INPROC_SERVER)? };

    // ── 3. Auto-detect format and create a decoder ────────────────────
    let decoder = unsafe {
        factory
            .CreateDecoderFromStream(&stream, None, WICDecodeOptions(0))
            .map_err(|e| wic_error("CreateDecoderFromStream", e))?
    };

    // ── 4. Get the first (and usually only) frame ─────────────────────
    let frame = unsafe { decoder.GetFrame(0).map_err(|e| wic_error("GetFrame", e))? };

    // ── 5. Check dimensions before decoding ────────────────────────────
    let (w, h) = unsafe { get_frame_size(&frame)? };
    if w > limits::MAX_IMAGE_DIMENSION || h > limits::MAX_IMAGE_DIMENSION {
        return Err(format!(
            "WIC image dimensions too large: {w}x{h} (max {})",
            limits::MAX_IMAGE_DIMENSION
        )
        .into());
    }
    let pixel_bytes = (w as u64).saturating_mul(h as u64).saturating_mul(4);
    if pixel_bytes > limits::MAX_IMAGE_ALLOC {
        return Err(format!(
            "WIC decoded buffer would exceed allocation limit: {pixel_bytes} bytes"
        )
        .into());
    }

    // ── 6. Convert to 32bpp RGBA via format converter ─────────────────
    let converter: IWICFormatConverter = unsafe {
        factory
            .CreateFormatConverter()
            .map_err(|e| wic_error("CreateFormatConverter", e))?
    };
    unsafe {
        converter
            .Initialize(
                &frame,
                &GUID_WICPixelFormat32bppRGBA,
                WICBitmapPaletteType(0),
                None,
                0.0,
                WICPixelFormatTranslate(0),
            )
            .map_err(|e| wic_error("Initialize format converter", e))?
    };

    // ── 7. Copy pixels into a Vec<u8> ─────────────────────────────────
    let stride = (w as u32 * 4).align_to::<u32>().padding;
    let buf_len = stride * h;
    let mut buf = vec![0u8; buf_len as usize];
    unsafe {
        converter
            .CopyPixels(
                None,
                stride,
                buf_len,
                Some(buf.as_mut_ptr()),
            )
            .map_err(|e| wic_error("CopyPixels", e))?
    };

    // ── 8. Build image::RgbaImage ────────────────────────────────────
    // WIC returns bottom-up rows when stride > width*4 is not zero.
    // For 32bpp RGBA, stride equals width*4 exactly (width is always
    // 4-byte aligned for the WIC pixel format), so the buffer is
    // tightly packed and top-down.
    let img = ImageBuffer::<Rgba<u8>, Vec<u8>>::from_raw(w, h, buf)
        .ok_or("WIC output buffer size mismatch")?;

    Ok(DynamicImage::ImageRgba8(img))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Wrap a byte slice in an HGLOBAL-backed COM `IStream` that WIC can
/// read from.
fn create_stream_on_hglobal(bytes: &[u8]) -> Result<IStream, Box<dyn Error>> {
    let mut stream: Option<IStream> = None;
    // CreateStreamOnHGlobal(NULL, TRUE) allocates an HGLOBAL internally
    // and will free it when the IStream is released.  We then write the
    // bytes via IStream::Write and seek back to the start.
    unsafe {
        CreateStreamOnHGlobal(None, true, &mut stream)
            .map_err(|e| wic_error("CreateStreamOnHGlobal", e))?;
    }
    let stream = stream.ok_or("CreateStreamOnHGlobal returned null")?;

    // Write image data into the stream.
    unsafe {
        let written = stream.Write(
            bytes.as_ptr() as *const _,
            bytes.len() as u32,
            None,
        );
        written.map_err(|e| wic_error("IStream::Write", e))?;
    }

    // Rewind to the beginning so WIC sees the full data from the start.
    unsafe {
        use windows::Win32::System::Com::STREAM_SEEK_SET;
        stream
            .Seek(0, STREAM_SEEK_SET, None)
            .map_err(|e| wic_error("IStream::Seek", e))?;
    }

    Ok(stream)
}

/// Query a WIC bitmap frame for its pixel dimensions.
unsafe fn get_frame_size(frame: &IWICBitmapFrameDecode) -> Result<(u32, u32), Box<dyn Error>> {
    let mut w = 0u32;
    let mut h = 0u32;
    unsafe {
        frame
            .GetSize(&mut w, &mut h)
            .map_err(|e| wic_error("GetSize", e))?;
    }
    Ok((w, h))
}

/// Wrap a WIC error into a `Box<dyn Error>` with a descriptive prefix.
fn wic_error(context: &str, hr: windows::core::Error) -> Box<dyn Error> {
    // WINCODEC_ERR_COMPONENTNOTFOUND = 0x88982F50
    if hr.code().0 == 0x88982F50_u32 as i32 {
        format!("{context}: no WIC codec installed for this image format").into()
    } else {
        format!("{context}: {hr}").into()
    }
}

/// `CoCreateInstance` wrapper that hides the unsafe + GUID boilerplate.
///
/// In the `windows` crate 0.62, `CoCreateInstance` is typically available
/// as `windows::Win32::System::Com::CoCreateInstance`.  If the exact
/// function signature differs in your build, an alternative pattern is:
///
/// ```ignore
/// use windows::Win32::System::Com::{CoCreateInstance, CLSCTX_INPROC_SERVER};
/// let factory: IWICImagingFactory2 =
///     CoCreateInstance(&CLSID_WICImagingFactory2, None, CLSCTX_INPROC_SERVER)?;
/// ```
unsafe fn CoCreateInstance<T: Interface>(
    clsid: *const windows::core::GUID,
    unk_outer: Option<*mut core::ffi::c_void>,
    cls_ctx: u32,
) -> Result<T, windows::core::Error> {
    let mut result: Option<T> = None;
    windows::Win32::System::Com::CoCreateInstance(
        clsid,
        unk_outer,
        windows::Win32::System::Com::CLSCTX(cls_ctx),
    )
    .and_then(|v: core::ffi::c_void| {
        // CoCreateInstance returns a raw pointer; cast it into the
        // requested interface via QueryInterface.
        // NOTE: The windows crate 0.62 CoCreateInstance actually returns
        // the interface directly when generic. If the above doesn't compile,
        // replace with the two-arg form:
        //
        //   let factory: IWICImagingFactory2 = unsafe {
        //       CoCreateInstance(
        //           &CLSID_WICImagingFactory2,
        //           None,
        //           CLSCTX_INPROC_SERVER,
        //       )?
        //   };
        T::from_raw(std::ptr::NonNull::new_unchecked(v as *mut _))
    })
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn is_wic_format_recognises_avif() {
        assert!(is_wic_format("cover.avif"));
        assert!(is_wic_format("COVER.AVIF"));
        assert!(is_wic_format("path/to/image.AViF"));
    }

    #[test]
    fn is_wic_format_recognises_jxl() {
        assert!(is_wic_format("photo.jxl"));
        assert!(is_wic_format("PHOTO.JXL"));
        assert!(is_wic_format("folder/page.JxL"));
    }

    #[test]
    fn is_wic_format_rejects_other_formats() {
        assert!(!is_wic_format("photo.jpg"));
        assert!(!is_wic_format("photo.png"));
        assert!(!is_wic_format("photo.webp"));
        assert!(!is_wic_format("photo.avif.txt"));  // suffix match only
        assert!(!is_wic_format(""));
    }
}
```

### 3.3 注意事项（编辑时必须处理）

**`create_stream_on_hglobal` 中的 `CreateStreamOnHGlobal` 调用：**

`windows` crate 0.62 中 `CreateStreamOnHGlobal` 的签名可能是以下两种之一（需 `cargo check` 确认）：

```rust
// 形式 A（返回 Result<IStream>）：
unsafe fn CreateStreamOnHGlobal(
    hglobal: *mut core::ffi::c_void,  // None 表示自动分配
    fdelete_on_release: bool,
) -> Result<IStream>

// 形式 B（输出参数）：
unsafe fn CreateStreamOnHGlobal(
    hglobal: *mut core::ffi::c_void,
    fdelete_on_release: BOOL,
    ppstm: *mut Option<IStream>,
) -> HRESULT
```

上面的代码使用的是**形式 B**。如果实际是**形式 A**，请改为：

```rust
fn create_stream_on_hglobal(bytes: &[u8]) -> Result<IStream, Box<dyn Error>> {
    let mut stream = unsafe {
        CreateStreamOnHGlobal(None, true)
            .map_err(|e| wic_error("CreateStreamOnHGlobal", e))?
    };
    // ... 后续 Write / Seek 不变 ...
}
```

**`CoCreateInstance` 签名：**

`windows` crate 0.62 的 `CoCreateInstance` 通常是泛型形式，直接返回接口类型：

```rust
let factory: IWICImagingFactory2 = unsafe {
    CoCreateInstance(
        &CLSID_WICImagingFactory2,
        None,
        CLSCTX_INPROC_SERVER,
    )?
};
```

如果泛型形式可用，删除上面代码中手写的 `CoCreateInstance` wrapper 函数，
直接在 `decode_via_wic` 第 2 步使用这一行即可。

需要额外导入：
```rust
use windows::Win32::System::Com::{CoCreateInstance, CLSCTX_INPROC_SERVER};
```

**`CLSCTX_INPROC_SERVER`** 可能在 `windows::Win32::System::Com` 中是 `CLSCTX(u32)` 类型或常量。如果 `CLSCTX_INPROC_SERVER` 不存在，用 `CLSCTX(1u32)` 代替（值为 `CLSCTX_INPROC_SERVER`）。

**`CopyPixels` 签名：**

`IWICBitmapSource::CopyPixels` 在 `windows` crate 中可能需要 `WICRect` 参数。如果第一个参数不能为 `None`（需要矩形），则改为：

```rust
use windows::Win32::Graphics::Imaging::WICRect;

let rect = WICRect {
    X: 0,
    Y: 0,
    Width: w as i32,
    Height: h as i32,
};
unsafe {
    converter
        .CopyPixels(
            Some(&rect),
            stride,
            buf_len,
            Some(buf.as_mut_ptr()),
        )
        .map_err(|e| wic_error("CopyPixels", e))?
};
```

**`stride` 计算简化：**

由于 `GUID_WICPixelFormat32bppRGBA` 每像素恰好 4 字节，且 WIC 保证 stride = width × 4（无填充），可以简化为：

```rust
let stride = w * 4;
let buf_len = stride * h;
```

如果编译器报错说 `align_to()` 不是 `u32` 的方法（因为 `u32` 没有这个方法），就直接用这个简化版本。

**`IStream::Write` 和 `IStream::Seek` 的返回类型：**

`IStream::Write` 在 `windows` crate 中可能返回 `Result<u32>` 或 `Result<()>`。
`IStream::Seek` 可能返回 `Result<u64>` 或 `Result<()>`。
请根据编译错误调整 `.map_err(...)` 和对返回值的使用。

**`WICBitmapPaletteType(0)` 和 `WICPixelFormatTranslate(0)`：**

如果这些是枚举而不是 newtype，请改用 `WICBitmapPaletteType::WICBitmapPaletteTypeCustom`
或对应的零值变体。同样 `WICDecodeOptions(0)` 可能需要是 `WICDecodeOptions::WICDecodeOptionsCacheMetadata`
或其他零值变体。

**`WINCODEC_ERR_COMPONENTNOTFOUND` 的 HRESULT 值：**

值 `0x88982F50` 需确认。如果 `windows::Win32::Graphics::Imaging` 导出了
`WINCODEC_ERR_COMPONENTNOTFOUND` 常量，则直接使用它替代硬编码数字。

---

## 4. 待修改文件：`src/lib.rs`

### 4.1 修改位置

在第 21 行 `mod stream;` 之后添加条件模块声明。

### 4.2 具体变更

```diff
 mod stream;
+#[cfg(feature = "wic")]
+mod wic;
```

---

## 5. 待修改文件：`src/decode.rs`

### 5.1 修改总览

1. 更新模块文档（第 1–18 行）
2. 删除 `#[cfg(feature = "jxl")] use image::ImageDecoder;`（第 23–24 行）
3. 删除 `decode_with_limits` 中的 jxl 分支（第 39–43 行）
4. 删除 `decode_for_thumbnail` 中的 jxl 分支（第 64–68 行）
5. 删除整个 `decode_jxl` 函数（第 186–204 行）
6. 添加 `#[cfg(feature = "wic")] use crate::wic;`
7. 在 `decode_with_limits` 和 `decode_for_thumbnail` 中添加 wic 分支

### 5.2 具体变更

#### 5.2.1 更新模块文档（替换第 1–18 行）

```rust
//! Image decoding with format-specific dispatch.
//!
//! Most formats (JPEG/PNG/GIF/BMP/TIFF/ICO/WebP) go through the
//! `image` crate's `ImageReader`, which auto-detects format from
//! magic bytes and enforces pre-decode dimension/allocation limits.
//!
//! **AVIF and JXL** (JPEG XL) are gated behind the `wic` Cargo feature.
//! When enabled, decoding uses the Windows Imaging Component (WIC) —
//! a native COM framework that delegates to system-installed codecs.
//! Windows 11 24H2+ ships AVIF and JXL codecs built-in; Windows 10
//! requires installing them from the Microsoft Store. Build with
//! `cargo build --release --features wic` to enable.
```

#### 5.2.2 删除第 23–24 行

```diff
-#[cfg(feature = "jxl")]
-use image::ImageDecoder;
```

#### 5.2.3 添加 wic import（在 `use crate::limits;` 之后）

```diff
 use crate::limits;
+#[cfg(feature = "wic")]
+use crate::wic;
```

#### 5.2.4 替换 `decode_with_limits` 函数（第 38–45 行）

```rust
pub fn decode_with_limits(name: &str, bytes: &[u8]) -> Result<DynamicImage, Box<dyn Error>> {
    #[cfg(feature = "wic")]
    if wic::is_wic_format(name) {
        return wic::decode_via_wic(bytes);
    }
    decode_via_image_crate(bytes)
}
```

#### 5.2.5 替换 `decode_for_thumbnail` 函数（第 59–74 行）

```rust
pub fn decode_for_thumbnail(
    name: &str,
    bytes: &[u8],
    target_px: u32,
) -> Result<DynamicImage, Box<dyn Error>> {
    #[cfg(feature = "wic")]
    if wic::is_wic_format(name) {
        return wic::decode_via_wic(bytes);
    }

    if let Some(img) = try_decode_jpeg_scaled(bytes, target_px)? {
        return Ok(img);
    }
    decode_via_image_crate(bytes)
}
```

#### 5.2.6 删除整个 `decode_jxl` 函数（第 186–204 行）

删除以下完整代码块：

```rust
#[cfg(feature = "jxl")]
fn decode_jxl(bytes: &[u8]) -> Result<DynamicImage, Box<dyn Error>> {
    use jxl_oxide::integration::JxlDecoder;

    let decoder = JxlDecoder::new(Cursor::new(bytes))?;
    let (w, h) = decoder.dimensions();

    // Pre-decode size guard — `image::Limits` isn't honoured by every
    // third-party decoder, so we check manually before committing.
    if w > limits::MAX_IMAGE_DIMENSION || h > limits::MAX_IMAGE_DIMENSION {
        return Err(format!("JXL dimensions too large: {w}x{h}").into());
    }
    let pixel_bytes = (w as u64).saturating_mul(h as u64).saturating_mul(4);
    if pixel_bytes > limits::MAX_IMAGE_ALLOC {
        return Err(format!("JXL would allocate {pixel_bytes} bytes, exceeds limit").into());
    }

    Ok(DynamicImage::from_decoder(decoder)?)
}
```

---

## 6. 待修改文件：`src/settings.rs`

### 6.1 修改位置

`SUPPORTED_IMAGE_EXTS` 数组（第 104–116 行）。

### 6.2 关键约束

**必须在数组末尾追加，不能插入中间。** 因为 `enabled_image_exts_mask` 是按位映射的，
位 `i` 对应索引 `i` 的扩展名。在中间插入会改变后续所有扩展名的 bit 位置，破坏向后兼容。

### 6.3 具体变更

```diff
 pub const SUPPORTED_IMAGE_EXTS: &[&str] = &[
     ".jpg",
     ".jpeg",
     ".png",
     ".gif",
     ".bmp",
     ".tiff",
     ".tif",
     ".webp",
     ".ico",
-    #[cfg(feature = "jxl")]
-    ".jxl",
+    #[cfg(feature = "wic")]
+    ".avif",
+    #[cfg(feature = "wic")]
+    ".jxl",
 ];
```

**注意**：`.avif` 在 `.jxl` 前面（字母序），但更重要的是两者都在 `.ico` 之后、
数组末尾。先加 `.avif` 再加 `.jxl` 意味着：
- 旧 build（无 wic feature）：mask bit 9 对应 `.ico`，9 位有效
- 新 build（有 wic feature）：mask bit 9 = `.avif`，bit 10 = `.jxl`，11 位有效

这与 `default_enabled_image_exts_mask()` 自动根据数组长度计算的正确行为一致。

---

## 7. 待修改文件：`.github/workflows/ci.yml`

### 7.1 修改位置

第 47–51 行。

### 7.2 具体变更

```diff
-      # The `jxl` feature is off by default, so the steps above never
-      # compile decode.rs's jxl-oxide path. Build and test it explicitly
-      # here, otherwise a breaking jxl-oxide bump passes CI unnoticed.
-      - name: Test (jxl feature)
-        run: cargo test --features jxl
+      # The `wic` feature is off by default, so the steps above never
+      # compile decode.rs's WIC path. Build and test it explicitly
+      # here, otherwise a breaking windows-rs WIC API change passes CI
+      # unnoticed.
+      - name: Test (wic feature)
+        run: cargo test --features wic
```

---

## 8. 待修改文件：`README.md`

### 8.1 修改位置

第 41 行 "Image formats inside archives" 段落。

### 8.2 具体变更

替换第 41 行：

```diff
-JPEG, PNG, GIF, BMP, TIFF, ICO, and WebP. Each format can be individually enabled or disabled in the configuration GUI. AVIF, HEIC, and SVG are not supported yet, mostly because their reference decoders pull in heavy C dependencies.
+JPEG, PNG, GIF, BMP, TIFF, ICO, WebP, AVIF, and JXL. Each format can be individually enabled or disabled in the configuration GUI. AVIF and JXL decoding uses the Windows Imaging Component (WIC) and requires system-installed codecs — Windows 11 24H2+ includes them by default; Windows 10 needs the [AV1 Image Extensions](https://apps.microsoft.com/detail/9n26s50ln705) and/or [JPEG XL Extensions](https://apps.microsoft.com/detail/9n8s2p8p2p2p) from the Microsoft Store. Enable with `cargo build --release --features wic`. HEIC and SVG are not supported yet.
```

### 8.3 "Known limitations" 段落（第 228 行）

```diff
-- AVIF, HEIC, SVG, and DjVu are not supported.
+- HEIC, SVG, and DjVu are not supported.
```

### 8.4 "Credits" 段落（第 242 行）

无需修改，WIC 是 Windows 系统组件，不算第三方依赖。

---

## 9. 验证清单

完成所有修改后，依次执行：

```powershell
# 1. 默认 feature（无 wic）—— 必须编译通过，零新依赖
cargo check
cargo test
cargo clippy --all-targets -- -D warnings

# 2. 启用 wic feature
cargo check --features wic
cargo test --features wic
cargo clippy --all-targets --features wic -- -D warnings

# 3. release 构建
cargo build --release
cargo build --release --features wic
```

**常见编译问题及修复方向：**

| 错误 | 原因 | 修复 |
|------|------|------|
| `CreateStreamOnHGlobal` 参数数量不匹配 | windows-rs 版本差异 | 参见 §3.3 中的形式 A/B |
| `CoCreateInstance` 泛型不可用 | 某些 windows 版本不支持泛型形式 | 改用 `CoCreateInstance` + `QueryInterface` 手动模式 |
| `CLSCTX_INPROC_SERVER` 不存在 | 可能在不同模块中 | 用 `windows::Win32::System::Com::CLSCTX(1u32)` |
| `CopyPixels` 第一个参数不能 `None` | 需要 `WICRect` | 传入 `Some(&WICRect { X:0, Y:0, Width:w as i32, Height:h as i32 })` |
| `WICDecodeOptions(0)` 不是 newtype | 可能是枚举 | 用零值变体如 `WICDecodeOptions::Default` |
| `align_to()` 不是 `u32` 方法 | Rust 2024 可能没有 | 直接用 `stride = w * 4` |
| 未使用 `name` 变量的警告 | wic 关闭时 name 未使用 | 加回 `let _ = name;` 在 wic cfg 之后 |
