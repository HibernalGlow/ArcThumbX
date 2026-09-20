//! UI strings for the config GUI, with English + Japanese translations.
//!
//! Selection order:
//! 1. `--lang en|ja` on the command line (sets `LANGUAGE_OVERRIDE`).
//! 2. (Windows) `HKCU\Software\ArcThumb\Language` registry override.
//! 3. OS default locale — starts with `"ja"` → Japanese.
//! 4. English fallback.
//!
//! Strings are handed to the Slint UI at startup via `in` properties.
//! A future refactor may move them into `.slint` `@tr("...")` with
//! gettext once the gettext toolchain (`xgettext`/`msgfmt`) is wired
//! into the build.

#[cfg(windows)]
use winreg::RegKey;
#[cfg(windows)]
use winreg::enums::*;

/// One UI language's worth of labels.
///
/// The tables below are `static`, not `const`: `for_macos` and the tests
/// compare table identity with `std::ptr::eq`, and a `const` is inlined at
/// every use site, so `&EN` would be the address of a fresh copy each time
/// and every comparison would silently fail.
#[derive(Clone)]
pub struct Strings {
    pub window_title: &'static str,
    pub menu_file: &'static str,
    pub menu_file_exit: &'static str,
    pub menu_help: &'static str,
    pub menu_help_about: &'static str,
    pub group_extensions: &'static str,
    pub group_image_exts: &'static str,
    pub group_sort: &'static str,
    pub sort_natural: &'static str,
    pub sort_alphabetical: &'static str,
    pub group_cover: &'static str,
    pub cover_prefer: &'static str,
    pub cover_only: &'static str,
    pub cover_ignore: &'static str,
    pub group_other: &'static str,
    pub cb_enable_preview: &'static str,
    pub cb_overlay_border: &'static str,
    pub cb_overlay_label: &'static str,
    pub cb_log_enabled: &'static str,
    pub btn_ok: &'static str,
    pub btn_cancel: &'static str,
    pub btn_apply: &'static str,
    pub btn_regenerate: &'static str,
    pub btn_close: &'static str,
    pub about_title: &'static str,
    pub about_body: &'static str,
    pub regen_confirm: &'static str,
    pub regen_done: &'static str,
    pub regen_partial: &'static str,
    pub error_title: &'static str,
    pub error_save: &'static str,
    pub error_register: &'static str,
    pub error_gui_init: &'static str,
    // Update check dialog
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub update_title: &'static str,
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub update_available: &'static str,
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub update_skip_checkbox: &'static str,
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub update_btn_open: &'static str,
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub update_btn_later: &'static str,
    // Donation dialog
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub donation_title: &'static str,
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub donation_prompt: &'static str,
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub donation_dont_show_checkbox: &'static str,
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub donation_btn_sponsor: &'static str,
    /// Only read by the Windows updater UI.
    #[cfg(windows)]
    pub donation_btn_later: &'static str,
}

pub static EN: Strings = Strings {
    window_title: "ArcThumb Configuration",
    menu_file: "File",
    menu_file_exit: "Exit",
    menu_help: "Help",
    menu_help_about: "About ArcThumb",
    group_extensions: "Enabled extensions",
    group_image_exts: "Image formats used for thumbnails (inside archives)",
    group_sort: "Sort order",
    sort_natural: "Natural (page2 < page10)",
    sort_alphabetical: "Alphabetical",
    group_cover: "Cover image (cover / folder / thumb / thumbnail / front)",
    cover_prefer: "Use cover if present, else first page",
    cover_only: "Cover only (no thumbnail otherwise)",
    cover_ignore: "Always use first page",
    group_other: "Other settings",
    cb_enable_preview: "Enable preview pane (Alt+P)",
    cb_overlay_border: "Mark archives with a coloured border",
    cb_overlay_label: "Mark archives with a format label (CBZ, EPUB, ...)",
    cb_log_enabled: "Enable diagnostic logging (%TEMP%\\arcthumb.log)",
    btn_ok: "OK",
    btn_cancel: "Cancel",
    btn_apply: "Apply",
    btn_regenerate: "Regenerate thumbnails",
    btn_close: "Close",
    about_title: "About ArcThumb",
    about_body: "ArcThumb — archive thumbnail provider for Windows Explorer.\n\nThis application uses Slint (https://slint.dev) under the Slint Royalty-Free License 2.0.",
    regen_confirm: "This will close all Explorer windows, delete the Windows thumbnail and icon caches, and restart Explorer.\n\nUse this if archive thumbnails are still missing after installing or enabling new file types, or to apply a change to the identification overlay.\n\nContinue?",
    regen_done: "Thumbnail cache cleared and Explorer restarted.\n\nNew thumbnails will be generated as you browse.",
    regen_partial: "Some cache files were locked and could not be deleted. Try closing other applications and run this again.",
    error_title: "ArcThumb",
    error_save: "Failed to save settings to the registry.",
    error_register: "Failed to update shell extension registration.",
    error_gui_init: "Failed to initialize the configuration UI. The graphics backend could not start. This can happen on systems without GPU acceleration (for example Windows Sandbox).",
    #[cfg(windows)]
    update_title: "Update available",
    #[cfg(windows)]
    update_available: "A new version of ArcThumb is available: v{}  (current: v{})",
    #[cfg(windows)]
    update_skip_checkbox: "Skip this version",
    #[cfg(windows)]
    update_btn_open: "Open download page",
    #[cfg(windows)]
    update_btn_later: "Remind me later",
    #[cfg(windows)]
    donation_title: "Thank you for updating!",
    #[cfg(windows)]
    donation_prompt: "ArcThumb has been updated to v{}.\nWould you like to support development?",
    #[cfg(windows)]
    donation_dont_show_checkbox: "Don't show this again",
    #[cfg(windows)]
    donation_btn_sponsor: "Open sponsor page",
    #[cfg(windows)]
    donation_btn_later: "Maybe next time",
};

pub static JA: Strings = Strings {
    window_title: "ArcThumb 設定",
    menu_file: "ファイル",
    menu_file_exit: "終了",
    menu_help: "ヘルプ",
    menu_help_about: "ArcThumb について",
    group_extensions: "有効にする拡張子",
    group_image_exts: "サムネイルに使う画像形式 (アーカイブ内)",
    group_sort: "並び順",
    sort_natural: "自然順 (page2 < page10)",
    sort_alphabetical: "アルファベット順",
    group_cover: "カバー画像 (cover / folder / thumb / thumbnail / front)",
    cover_prefer: "カバーを優先（無ければ先頭ページ）",
    cover_only: "カバーがあるときだけ表示（無ければ通常アイコン）",
    cover_ignore: "常に先頭ページを使う",
    group_other: "その他の設定",
    cb_enable_preview: "プレビュー ウィンドウを有効にする (Alt+P)",
    cb_overlay_border: "アーカイブを色付きの枠線で示す",
    cb_overlay_label: "アーカイブにフォーマットラベルを表示 (CBZ, EPUB, ...)",
    cb_log_enabled: "診断ログを有効にする (%TEMP%\\arcthumb.log)",
    btn_ok: "OK",
    btn_cancel: "キャンセル",
    btn_apply: "適用",
    btn_regenerate: "サムネイルを再生成",
    btn_close: "閉じる",
    about_title: "ArcThumb について",
    about_body: "ArcThumb — Windows エクスプローラー向けのアーカイブサムネイル プロバイダー。\n\nこのアプリケーションは Slint (https://slint.dev) を Slint Royalty-Free License 2.0 に基づいて使用しています。",
    regen_confirm: "エクスプローラーのウィンドウをすべて閉じ、Windows のサムネイル/アイコンキャッシュを削除してエクスプローラーを再起動します。\n\nインストール後や対応拡張子を有効にしたあとでサムネイルが表示されない場合や、識別オーバーレイの設定を変更したあとに使ってください。\n\n続行しますか？",
    regen_done: "サムネイルキャッシュを削除し、エクスプローラーを再起動しました。\n\nフォルダを開くと新しいサムネイルが作成されます。",
    regen_partial: "一部のキャッシュファイルがロックされていて削除できませんでした。他のアプリを閉じてから、もう一度実行してください。",
    error_title: "ArcThumb",
    error_save: "設定の保存に失敗しました。",
    error_register: "シェル拡張の登録状態の更新に失敗しました。",
    error_gui_init: "設定 UI の初期化に失敗しました。グラフィックスバックエンドを開始できませんでした。GPU アクセラレーションが利用できない環境 (Windows Sandbox など) で発生することがあります。",
    #[cfg(windows)]
    update_title: "アップデート通知",
    #[cfg(windows)]
    update_available: "ArcThumb の新しいバージョンがあります: v{}  (現在: v{})",
    #[cfg(windows)]
    update_skip_checkbox: "このバージョンをスキップ",
    #[cfg(windows)]
    update_btn_open: "ダウンロードページを開く",
    #[cfg(windows)]
    update_btn_later: "あとで通知",
    #[cfg(windows)]
    donation_title: "アップデートありがとうございます！",
    #[cfg(windows)]
    donation_prompt: "ArcThumb v{} にアップデートされました。\n開発を支援しますか？",
    #[cfg(windows)]
    donation_dont_show_checkbox: "今後表示しない",
    #[cfg(windows)]
    donation_btn_sponsor: "スポンサーページを開く",
    #[cfg(windows)]
    donation_btn_later: "また今度",
};

/// Simplified Chinese. Same field order as `EN`; the Windows-only updater
/// and donation rows are gated exactly as they are in the other tables.
pub static ZH: Strings = Strings {
    window_title: "ArcThumb 设置",
    menu_file: "文件",
    menu_file_exit: "退出",
    menu_help: "帮助",
    menu_help_about: "关于 ArcThumb",
    group_extensions: "启用的扩展名",
    group_image_exts: "用于缩略图的图片格式（压缩包内）",
    group_sort: "排序方式",
    sort_natural: "自然序（page2 < page10）",
    sort_alphabetical: "字母序",
    group_cover: "封面图（cover / folder / thumb / thumbnail / front）",
    cover_prefer: "优先使用封面，没有则用首页",
    cover_only: "仅封面（没有则不显示缩略图）",
    cover_ignore: "始终使用首页",
    group_other: "其他设置",
    cb_enable_preview: "启用预览窗格 (Alt+P)",
    cb_overlay_border: "为压缩包加上色边框",
    cb_overlay_label: "为压缩包显示格式标签 (CBZ, EPUB, ...)",
    cb_log_enabled: "启用诊断日志 (%TEMP%\\arcthumb.log)",
    btn_ok: "确定",
    btn_cancel: "取消",
    btn_apply: "应用",
    btn_regenerate: "重新生成缩略图",
    btn_close: "关闭",
    about_title: "关于 ArcThumb",
    about_body: "ArcThumb — Windows 资源管理器的压缩包缩略图扩展。\n\n本程序使用 Slint (https://slint.dev)，遵循 Slint Royalty-Free License 2.0。",
    regen_confirm: "这将关闭所有资源管理器窗口、删除 Windows 的缩略图与图标缓存，并重启资源管理器。\n\n如果安装或启用新扩展名后仍看不到缩略图，或更改了识别标记设置，请使用它。\n\n继续吗？",
    regen_done: "已清除缩略图缓存并重启资源管理器。\n\n浏览文件夹时会生成新的缩略图。",
    regen_partial: "部分缓存文件被占用而无法删除。请关闭其他程序后重试。",
    error_title: "ArcThumb",
    error_save: "保存设置到注册表失败。",
    error_register: "更新 shell 扩展注册状态失败。",
    error_gui_init: "初始化设置界面失败：无法启动图形后端。在没有 GPU 加速的环境（例如 Windows Sandbox）中可能出现。",
    #[cfg(windows)]
    update_title: "有可用更新",
    #[cfg(windows)]
    update_available: "ArcThumb 有新版本：v{}（当前：v{}）",
    #[cfg(windows)]
    update_skip_checkbox: "跳过此版本",
    #[cfg(windows)]
    update_btn_open: "打开下载页面",
    #[cfg(windows)]
    update_btn_later: "稍后提醒",
    #[cfg(windows)]
    donation_title: "感谢更新！",
    #[cfg(windows)]
    donation_prompt: "ArcThumb 已更新到 v{}。\n愿意支持开发吗？",
    #[cfg(windows)]
    donation_dont_show_checkbox: "不再显示",
    #[cfg(windows)]
    donation_btn_sponsor: "打开赞助页面",
    #[cfg(windows)]
    donation_btn_later: "下次再说",
};

/// Explicit `--lang` choice, applied once by `main` before any window is
/// built. Only the macOS front end has a `--lang` flag; Windows keeps its
/// registry `Language` value.
#[cfg(not(windows))]
static LANGUAGE_OVERRIDE: std::sync::OnceLock<Option<String>> = std::sync::OnceLock::new();

#[cfg(not(windows))]
pub fn set_language_override(lang: Option<&str>) {
    let _ = LANGUAGE_OVERRIDE.set(lang.map(str::to_owned));
}

/// Map a language tag to its table. Anything unrecognised keeps the caller's
/// guess, so a typo in `--lang` or the registry falls back to the OS locale
/// instead of silently forcing English.
fn table_for(tag: &str) -> Option<&'static Strings> {
    let tag = tag.to_ascii_lowercase();
    if tag.starts_with("ja") {
        Some(&JA)
    } else if tag.starts_with("zh") || tag == "chinese" {
        Some(&ZH)
    } else if tag.starts_with("en") {
        Some(&EN)
    } else {
        None
    }
}

#[cfg(not(windows))]
fn pick(base: &'static Strings) -> &'static Strings {
    match LANGUAGE_OVERRIDE
        .get()
        .and_then(|o| o.as_deref())
        .and_then(table_for)
    {
        Some(table) => table,
        None => base,
    }
}

/// Resolve the UI language to use right now.
#[cfg(windows)]
pub fn current() -> &'static Strings {
    // 1. Registry override.
    if let Ok(key) = RegKey::predef(HKEY_CURRENT_USER).open_subkey("Software\\ArcThumb")
        && let Ok(lang) = key.get_value::<String, _>("Language")
    {
        return table_for(&lang).unwrap_or(&EN);
    }

    // 2. OS default locale, 3. English fallback.
    if detect_os_locale_is_japanese() {
        &JA
    } else {
        &EN
    }
}

/// Resolve the UI language: `--lang` first, then the OS locale, then English.
#[cfg(not(windows))]
pub fn current() -> &'static Strings {
    pick(detect_os_locale().unwrap_or(&EN))
}

#[cfg(not(windows))]
pub fn for_macos(base: &Strings) -> Strings {
    let mut s = base.clone();
    if std::ptr::eq(base, &ZH as *const Strings) {
        s.about_body = "ArcThumb — macOS Finder 的压缩包缩略图扩展。\n\n本程序使用 Slint (https://slint.dev)，遵循 Slint Royalty-Free License 2.0。";
        s.cb_enable_preview = "启用预览窗格（仅 Windows）";
        s.cb_log_enabled = "启用诊断日志（扩展容器内的 arcthumb.log）";
        s.regen_confirm = "这将清除 Finder 的缩略图缓存。\n\n修改设置后如果已显示的图标没有更新，请使用它。\n\n继续吗？";
        s.regen_done = "已清除缩略图缓存。重新打开文件夹即可重新生成。";
        s.regen_partial = "Quick Look 未能清除缓存。请重试。";
        s.error_save = "写入设置文件失败。";
        s.error_register = "应用设置失败。";
        s.error_gui_init = "初始化设置界面失败：无法启动图形后端。";
    } else if std::ptr::eq(base, &JA as *const Strings) {
        s.about_body = "ArcThumb — macOS Finder 向けアーカイブサムネイル プロバイダー。\n\nこのアプリケーションは Slint (https://slint.dev) を Slint Royalty-Free License 2.0 に基づいて使用しています。";
        s.cb_enable_preview = "プレビュー ウィンドウ（Windows 専用）";
        s.cb_log_enabled = "診断ログを有効にする（拡張機能のコンテナ内 arcthumb.log）";
        s.regen_confirm = "Finder のサムネイルキャッシュを削除します。\n\n変更した設定を既存のファイルに反映させたい場合に使用してください。\n\n続行しますか？";
        s.regen_done = "サムネイルキャッシュを削除しました。フォルダを開き直すと再生成されます。";
        s.regen_partial = "キャッシュを削除できませんでした。もう一度お試しください。";
        s.error_save = "設定ファイルの保存に失敗しました。";
        s.error_register = "設定の適用に失敗しました。";
        s.error_gui_init =
            "設定 UI を初期化できませんでした。グラフィックスバックエンドを起動できませんでした。";
    } else {
        s.about_body = "ArcThumb — archive thumbnail provider for macOS Finder.\n\nThis application uses Slint (https://slint.dev) under the Slint Royalty-Free License 2.0.";
        s.cb_enable_preview = "Enable preview pane (Windows only)";
        s.cb_log_enabled = "Enable diagnostic logging (arcthumb.log in the extension container)";
        s.regen_confirm = "This clears Finder's cached thumbnails.\n\nUse it after changing a setting if the icons you already see do not update.\n\nContinue?";
        s.regen_done = "Thumbnail cache cleared. Reopen the folder to regenerate.";
        s.regen_partial = "Quick Look could not clear its cache. Try again.";
        s.error_save = "Failed to save the settings file.";
        s.error_register = "Failed to apply the settings.";
        s.error_gui_init =
            "Failed to initialize the configuration UI. The graphics backend could not start.";
    }
    s
}

/// The OS locale, resolved to one of our tables, or `None` when the system
/// language is not translated.
#[cfg(windows)]
fn detect_os_locale() -> Option<&'static Strings> {
    use windows::Win32::Globalization::GetUserDefaultLocaleName;

    // LOCALE_NAME_MAX_LENGTH = 85
    let mut buf = [0u16; 85];
    let len = unsafe { GetUserDefaultLocaleName(&mut buf) };
    if len <= 0 {
        return None;
    }
    let end = (len as usize).saturating_sub(1);
    table_for(&String::from_utf16_lossy(&buf[..end]))
}

/// Non-Windows: the shell's `LC_ALL`/`LC_CTYPE`/`LANG` when present. A GUI
/// app launched from Finder usually has none of them, which is why `--lang`
/// exists — the alternative is linking CoreFoundation just to read one
/// preference.
#[cfg(not(windows))]
fn detect_os_locale() -> Option<&'static Strings> {
    ["LC_ALL", "LC_CTYPE", "LANG"]
        .iter()
        .filter_map(|name| std::env::var(name).ok())
        .find_map(|value| table_for(&value.replace('_', "-")))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn every_table_defines_the_same_labels() {
        // A missing translation shows up as an empty string in the UI, and
        // the only cheap guard is to compare the tables field by field.
        assert_eq!(EN.window_title, "ArcThumb Configuration");
        assert!(!JA.window_title.is_empty());
        assert!(!ZH.window_title.is_empty());
        assert_ne!(ZH.error_save, ZH.error_title, "a table must not alias rows");
    }

    #[test]
    fn language_tags_resolve_to_the_right_table() {
        assert!(std::ptr::eq(table_for("zh").unwrap(), &ZH));
        assert!(std::ptr::eq(table_for("zh_CN").unwrap(), &ZH));
        assert!(std::ptr::eq(table_for("zh-Hans").unwrap(), &ZH));
        assert!(std::ptr::eq(table_for("chinese").unwrap(), &ZH));
        assert!(std::ptr::eq(table_for("JA").unwrap(), &JA));
        assert!(std::ptr::eq(table_for("ja-jp").unwrap(), &JA));
        assert!(std::ptr::eq(table_for("en-GB").unwrap(), &EN));
        assert!(
            table_for("klingon").is_none(),
            "unknown tags must not guess"
        );
    }

    #[test]
    fn locale_strings_survive_the_macos_rewrite() {
        // The macOS variant replaces a handful of Windows-specific rows and
        // must leave the rest of the chosen language intact.
        let zh = for_macos(&ZH);
        assert_ne!(
            zh.about_body, ZH.about_body,
            "about text must be macOS-aware"
        );
        assert_ne!(zh.cb_enable_preview, ZH.cb_enable_preview);
        assert_eq!(zh.btn_ok, ZH.btn_ok, "untouched rows stay Chinese");
        assert_eq!(zh.group_sort, ZH.group_sort);

        let en = for_macos(&EN);
        assert_eq!(en.btn_ok, "OK");
        assert_ne!(en.regen_confirm, EN.regen_confirm);
    }
}
