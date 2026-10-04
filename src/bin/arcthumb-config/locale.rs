//! UI strings for the config panel, plus the language/theme preference.
//!
//! Three languages ship: Simplified Chinese, English, Japanese. Resolution
//! order, first match wins:
//!
//! 1. (macOS) `--lang en|ja|zh` on the command line.
//! 2. The persisted choice: `HKCU\Software\ArcThumb\Language` on Windows, the
//!    `language` key in the extension's settings file on macOS. Written by the
//!    language keycaps in the panel's top strip.
//! 3. The OS locale (`GetUserDefaultLocaleName` / `AppleLocale`), when it maps
//!    to one of the three.
//! 4. [`Locale::DEFAULT`] — Simplified Chinese.
//!
//! Step 4 is a deliberate fork change: upstream falls back to English, which
//! was right for a Japanese-authored extension aimed at Windows users
//! everywhere. Unrecognised OS languages (a German system, say) now land in
//! Chinese rather than English, and the keycaps are the way back.
//!
//! Every field of [`Strings`] is a plain `&'static str`, so a missing
//! translation in one table is a compile error rather than a blank label —
//! the tables are struct literals, and the compiler requires all fields. The
//! tests below cover what the compiler cannot: rows that are present but
//! empty, or present but still carrying the English text.
//!
//! The Silk-Screen captions (``EXT``, ``IMG``, ``SORT``, ``READY``) are *not*
//! in this file. They are hardware engraving — graphic marks fixed to the
//! panel, the same in every language — and live as constants in
//! [`crate::app::view`].

/// The theme of the *chassis*. The screens inside it — the LCD status readout,
/// the CRT overlay — never follow this: a dark panel behind a lit display is
/// the one thing both themes agree on.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Theme {
    Dark,
    Light,
}

impl Theme {
    pub const DEFAULT: Theme = Theme::Dark;

    pub fn tag(self) -> &'static str {
        match self {
            Theme::Dark => "dark",
            Theme::Light => "light",
        }
    }

    pub fn from_tag(tag: &str) -> Option<Theme> {
        match tag.trim().to_ascii_lowercase().as_str() {
            "dark" => Some(Theme::Dark),
            "light" => Some(Theme::Light),
            _ => None,
        }
    }

    pub fn toggled(self) -> Theme {
        match self {
            Theme::Dark => Theme::Light,
            Theme::Light => Theme::Dark,
        }
    }
}

#[cfg(windows)]
use winreg::RegKey;
#[cfg(windows)]
use winreg::enums::*;

/// A language the panel ships in.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Locale {
    Zh,
    En,
    Ja,
}

impl Locale {
    /// Fallback when nothing above it in the resolution order matched.
    pub const DEFAULT: Locale = Locale::Zh;

    /// Everything the language keycaps can select, in keycap order.
    pub const ALL: [Locale; 3] = [Locale::Zh, Locale::En, Locale::Ja];

    /// The value written to the registry / settings file. Lower-case ASCII so
    /// a hand-edited preference is recognisable next to `SortOrder`.
    pub fn tag(self) -> &'static str {
        match self {
            Locale::Zh => "zh",
            Locale::En => "en",
            Locale::Ja => "ja",
        }
    }

    /// What the language calls itself. Never translated: a Chinese user looks
    /// for 「中文」, not for "中文" spelled in the active language.
    pub fn endonym(self) -> &'static str {
        match self {
            Locale::Zh => "中文",
            Locale::En => "English",
            Locale::Ja => "日本語",
        }
    }

    /// Resolve a language tag: `zh`, `zh-CN`, `zh_Hans_CN`, `chinese` and
    /// case variants all land on [`Locale::Zh`]. `None` means "not one of
    /// ours", which keeps an unrecognised OS locale from being read as an
    /// explicit choice.
    pub fn from_tag(tag: &str) -> Option<Locale> {
        let tag = tag.to_ascii_lowercase().replace('_', "-");
        if tag.starts_with("zh") || tag.starts_with("chinese") || tag == "cn" {
            Some(Locale::Zh)
        } else if tag.starts_with("ja") || tag.starts_with("japanese") {
            Some(Locale::Ja)
        } else if tag.starts_with("en") || tag.starts_with("english") {
            Some(Locale::En)
        } else {
            None
        }
    }
}

/// Which front end is drawing. The Explorer and Finder dialogs share every
/// label but a handful, and those handful say things like "Alt+P" or
/// "%TEMP%\arcthumb.log" that would be lies on the other platform.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Platform {
    // Only ever *constructed* by the Windows build, but the mapping below is
    // shared and exhaustive, so the variant is not dead — it is the other half
    // of one function that both front ends call.
    #[cfg_attr(not(windows), allow(dead_code))]
    Windows,
    MacOS,
}

/// One language's worth of labels. `Copy` because every field is a `&'static
/// str`, and the panel reads a table far more often than it changes one.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Strings {
    pub window_title: &'static str,
    pub btn_exit: &'static str,
    pub btn_about: &'static str,
    pub group_extensions: &'static str,
    pub hint_extensions: &'static str,
    pub group_image_exts: &'static str,
    pub hint_image_exts: &'static str,
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
    pub lcd_ready: &'static str,
    pub lcd_saved: &'static str,
    /// Shown on a control the other platform owns: the preview pane is an
    /// Explorer feature, so the Finder panel keeps the row visible, inert, and
    /// labelled instead of hiding it.
    pub tag_windows_only: &'static str,
    pub theme_dark: &'static str,
    pub theme_light: &'static str,
    // Update check dialog — only the Windows updater draws these.
    #[cfg(windows)]
    pub update_title: &'static str,
    #[cfg(windows)]
    pub update_available: &'static str,
    #[cfg(windows)]
    pub update_skip_checkbox: &'static str,
    #[cfg(windows)]
    pub update_btn_open: &'static str,
    #[cfg(windows)]
    pub update_btn_later: &'static str,
    // Donation dialog — same.
    #[cfg(windows)]
    pub donation_title: &'static str,
    #[cfg(windows)]
    pub donation_prompt: &'static str,
    #[cfg(windows)]
    pub donation_dont_show_checkbox: &'static str,
    #[cfg(windows)]
    pub donation_btn_sponsor: &'static str,
    #[cfg(windows)]
    pub donation_btn_later: &'static str,
}

impl Strings {
    /// The table for one language on one platform.
    pub fn resolve(locale: Locale, platform: Platform) -> Strings {
        let mut strings = match locale {
            Locale::Zh => ZH,
            Locale::En => EN,
            Locale::Ja => JA,
        };
        if platform == Platform::MacOS {
            strings.apply_macos(locale);
        }
        strings
    }

    #[cfg(test)]
    /// Every label, as (field name, value) pairs in declaration order. Only
    /// the tests read this; it exists so "translated but still English" and
    /// "forgot a row" are caught by one loop instead of forty assertions.
    #[allow(unused_mut)]
    fn rows(self) -> Vec<(&'static str, &'static str)> {
        let mut rows = [
            ("window_title", self.window_title),
            ("btn_exit", self.btn_exit),
            ("btn_about", self.btn_about),
            ("group_extensions", self.group_extensions),
            ("hint_extensions", self.hint_extensions),
            ("group_image_exts", self.group_image_exts),
            ("hint_image_exts", self.hint_image_exts),
            ("group_sort", self.group_sort),
            ("sort_natural", self.sort_natural),
            ("sort_alphabetical", self.sort_alphabetical),
            ("group_cover", self.group_cover),
            ("cover_prefer", self.cover_prefer),
            ("cover_only", self.cover_only),
            ("cover_ignore", self.cover_ignore),
            ("group_other", self.group_other),
            ("cb_enable_preview", self.cb_enable_preview),
            ("cb_overlay_border", self.cb_overlay_border),
            ("cb_overlay_label", self.cb_overlay_label),
            ("cb_log_enabled", self.cb_log_enabled),
            ("btn_ok", self.btn_ok),
            ("btn_cancel", self.btn_cancel),
            ("btn_apply", self.btn_apply),
            ("btn_regenerate", self.btn_regenerate),
            ("btn_close", self.btn_close),
            ("about_title", self.about_title),
            ("about_body", self.about_body),
            ("regen_confirm", self.regen_confirm),
            ("regen_done", self.regen_done),
            ("regen_partial", self.regen_partial),
            ("error_title", self.error_title),
            ("error_save", self.error_save),
            ("error_register", self.error_register),
            ("error_gui_init", self.error_gui_init),
            ("lcd_ready", self.lcd_ready),
            ("lcd_saved", self.lcd_saved),
            ("tag_windows_only", self.tag_windows_only),
            ("theme_dark", self.theme_dark),
            ("theme_light", self.theme_light),
        ]
        .to_vec();
        #[cfg(windows)]
        rows.extend([
            ("update_title", self.update_title),
            ("update_available", self.update_available),
            ("update_skip_checkbox", self.update_skip_checkbox),
            ("update_btn_open", self.update_btn_open),
            ("update_btn_later", self.update_btn_later),
            ("donation_title", self.donation_title),
            ("donation_prompt", self.donation_prompt),
            (
                "donation_dont_show_checkbox",
                self.donation_dont_show_checkbox,
            ),
            ("donation_btn_sponsor", self.donation_btn_sponsor),
            ("donation_btn_later", self.donation_btn_later),
        ]);
        rows
    }
}

pub static EN: Strings = Strings {
    window_title: "ArcThumb Configuration",
    btn_exit: "Exit",
    btn_about: "About",
    group_extensions: "Enabled extensions",
    hint_extensions: "Checked types get a thumbnail in the file manager.",
    group_image_exts: "Image formats used for thumbnails (inside archives)",
    hint_image_exts: "Which files inside an archive may be used as its picture.",
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
    about_body: "ArcThumb — archive thumbnail provider for Windows Explorer.\n\nBuilt with Dioxus (https://dioxuslabs.com), MIT licensed.",
    regen_confirm: "This will close all Explorer windows, delete the Windows thumbnail and icon caches, and restart Explorer.\n\nUse this if archive thumbnails are still missing after installing or enabling new file types, or to apply a change to the identification overlay.\n\nContinue?",
    regen_done: "Thumbnail cache cleared and Explorer restarted.\n\nNew thumbnails will be generated as you browse.",
    regen_partial: "Some cache files were locked and could not be deleted. Try closing other applications and run this again.",
    error_title: "ArcThumb",
    error_save: "Failed to save settings to the registry.",
    error_register: "Failed to update shell extension registration.",
    error_gui_init: "Failed to initialize the configuration UI. The webview could not start — on Windows that usually means the WebView2 Runtime is missing.",
    lcd_ready: "Ready",
    lcd_saved: "Saved",
    tag_windows_only: "Windows only",
    theme_dark: "Dark",
    theme_light: "Light",
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
    btn_exit: "終了",
    btn_about: "情報",
    group_extensions: "有効にする拡張子",
    hint_extensions: "チェックした拡張子がサムネイルを表示します。",
    group_image_exts: "サムネイルに使う画像形式 (アーカイブ内)",
    hint_image_exts: "アーカイブ内のどのファイルを画像にするかを決めます。",
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
    about_body: "ArcThumb — Windows エクスプローラー向けのアーカイブサムネイル プロバイダー。\n\nDioxus (https://dioxuslabs.com) で構築、MIT ライセンス。",
    regen_confirm: "エクスプローラーのウィンドウをすべて閉じ、Windows のサムネイル/アイコンキャッシュを削除してエクスプローラーを再起動します。\n\nインストール後や対応拡張子を有効にしたあとでサムネイルが表示されない場合や、識別オーバーレイの設定を変更したあとに使ってください。\n\n続行しますか？",
    regen_done: "サムネイルキャッシュを削除し、エクスプローラーを再起動しました。\n\nフォルダを開くと新しいサムネイルが作成されます。",
    regen_partial: "一部のキャッシュファイルがロックされていて削除できませんでした。他のアプリを閉じてから、もう一度実行してください。",
    error_title: "ArcThumb",
    error_save: "設定の保存に失敗しました。",
    error_register: "シェル拡張の登録状態の更新に失敗しました。",
    error_gui_init: "設定 UI を初期化できませんでした。Web ビューを起動できません — Windows では WebView2 ランタイムの未インストールが原因です。",
    lcd_ready: "準備完了",
    lcd_saved: "保存しました",
    tag_windows_only: "Windows のみ",
    theme_dark: "ダーク",
    theme_light: "ライト",
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

/// Simplified Chinese — and [`Locale::DEFAULT`], so this is what a system
/// language we do not translate falls back to.
pub static ZH: Strings = Strings {
    window_title: "ArcThumb 设置",
    btn_exit: "退出",
    btn_about: "关于",
    group_extensions: "启用的扩展名",
    hint_extensions: "勾选的扩展名会在文件管理器里显示缩略图。",
    group_image_exts: "用于缩略图的图片格式（压缩包内）",
    hint_image_exts: "决定压缩包里哪些文件可以当作封面图来源。",
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
    about_body: "ArcThumb — Windows 资源管理器的压缩包缩略图扩展。\n\n界面使用 Dioxus (https://dioxuslabs.com)，MIT 许可。",
    regen_confirm: "这将关闭所有资源管理器窗口、删除 Windows 的缩略图与图标缓存，并重启资源管理器。\n\n如果安装或启用新扩展名后仍看不到缩略图，或更改了识别标记设置，请使用它。\n\n继续吗？",
    regen_done: "已清除缩略图缓存并重启资源管理器。\n\n浏览文件夹时会生成新的缩略图。",
    regen_partial: "部分缓存文件被占用而无法删除。请关闭其他程序后重试。",
    error_title: "ArcThumb",
    error_save: "保存设置到注册表失败。",
    error_register: "更新 shell 扩展注册状态失败。",
    error_gui_init: "初始化设置界面失败：无法启动 WebView。在 Windows 上通常是缺少 WebView2 运行时。",
    lcd_ready: "就绪",
    lcd_saved: "已保存",
    tag_windows_only: "仅 Windows",
    theme_dark: "深色",
    theme_light: "浅色",
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

/// Rows that legitimately read the same in two languages, so the
/// "translated-but-still-English" test below must not flag them. The brand
/// name is a brand name; and Japanese conventionally leaves the confirm
/// button as `OK`.
#[cfg(test)]
const UNTRANSLATED_BY_DESIGN: &[&str] = &["error_title", "btn_ok"];

/// Rows the Finder front end rewrites, because the Windows wording names a
/// registry path, `Alt+P`, or Explorer itself. Anything outside this list must
/// read identically on both platforms — that is what keeps the shared panel
/// honest and stops a macOS tweak from silently changing the Windows dialog.
#[cfg(test)]
const MACOS_SPECIFIC: &[&str] = &[
    "about_body",
    "cb_enable_preview",
    "cb_log_enabled",
    "error_save",
    "error_register",
    "error_gui_init",
    "regen_confirm",
    "regen_done",
    "regen_partial",
];

impl Strings {
    /// Swap the rows that describe Windows hardware for the Finder truth.
    ///
    /// Keyed on the [`Locale`] rather than on `std::ptr::eq` of a table address:
    /// a table is a `static`, and comparing addresses to work out "which
    /// language am I" breaks the moment a table is cloned or moved.
    fn apply_macos(&mut self, locale: Locale) {
        match locale {
            Locale::Zh => {
                self.about_body = "ArcThumb — macOS Finder 的压缩包缩略图扩展。\n\n界面使用 Dioxus (https://dioxuslabs.com)，MIT 许可。";
                self.cb_enable_preview = "启用预览窗格（仅 Windows）";
                self.cb_log_enabled = "启用诊断日志（扩展容器内的 arcthumb.log）";
                self.regen_confirm = "这将清除 Finder 的缩略图缓存。\n\n修改设置后如果已显示的图标没有更新，请使用它。\n\n继续吗？";
                self.regen_done = "已清除缩略图缓存。重新打开文件夹即可重新生成。";
                self.regen_partial = "Quick Look 未能清除缓存。请重试。";
                self.error_save = "写入设置文件失败。";
                self.error_register = "应用设置失败。";
                self.error_gui_init = "初始化设置界面失败：无法启动 WebView。";
            }
            Locale::Ja => {
                self.about_body = "ArcThumb — macOS Finder 向けアーカイブサムネイル プロバイダー。\n\nDioxus (https://dioxuslabs.com) で構築、MIT ライセンス。";
                self.cb_enable_preview = "プレビュー ウィンドウ（Windows 専用）";
                self.cb_log_enabled = "診断ログを有効にする（拡張機能のコンテナ内 arcthumb.log）";
                self.regen_confirm = "Finder のサムネイルキャッシュを削除します。\n\n変更した設定を既存のファイルに反映させたい場合に使用してください。\n\n続行しますか？";
                self.regen_done =
                    "サムネイルキャッシュを削除しました。フォルダを開き直すと再生成されます。";
                self.regen_partial = "キャッシュを削除できませんでした。もう一度お試しください。";
                self.error_save = "設定ファイルの保存に失敗しました。";
                self.error_register = "設定の適用に失敗しました。";
                self.error_gui_init =
                    "設定 UI を初期化できませんでした。Web ビューを起動できません。";
            }
            Locale::En => {
                self.about_body = "ArcThumb — archive thumbnail provider for macOS Finder.\n\nBuilt with Dioxus (https://dioxuslabs.com), MIT licensed.";
                self.cb_enable_preview = "Enable preview pane (Windows only)";
                self.cb_log_enabled =
                    "Enable diagnostic logging (arcthumb.log in the extension container)";
                self.regen_confirm = "This clears Finder's cached thumbnails.\n\nUse it after changing a setting if the icons you already see do not update.\n\nContinue?";
                self.regen_done = "Thumbnail cache cleared. Reopen the folder to regenerate.";
                self.regen_partial = "Quick Look could not clear its cache. Try again.";
                self.error_save = "Failed to save the settings file.";
                self.error_register = "Failed to apply the settings.";
                self.error_gui_init =
                    "Failed to initialize the configuration UI. The webview could not start.";
            }
        }
    }
}

// =============================================================================
// Language / theme preference
//
// Both are GUI-only state, so they live next to each other rather than in the
// extension's settings model. Windows keeps them as two more `REG_SZ` values
// under the same key the extension already reads; macOS puts them in the same
// `key = value` file, where the extension ignores them.
// =============================================================================

/// Where `Language`/`Theme` live on Windows. Inlined rather than imported from
/// `arcthumb::settings`, whose copy of this path is a private constant.
#[cfg(windows)]
const PREF_SUBKEY: &str = "Software\\ArcThumb";

#[cfg(windows)]
fn read_pref(name: &str) -> Option<String> {
    let key = RegKey::predef(HKEY_CURRENT_USER)
        .open_subkey(PREF_SUBKEY)
        .ok()?;
    key.get_value::<String, _>(name).ok()
}

#[cfg(windows)]
fn write_pref(name: &str, value: &str) -> Result<(), String> {
    let hkcu = RegKey::predef(HKEY_CURRENT_USER);
    let (key, _) = hkcu
        .create_subkey(PREF_SUBKEY)
        .map_err(|e| format!("open {PREF_SUBKEY}: {e}"))?;
    key.set_value(name, &value)
        .map_err(|e| format!("write {name}: {e}"))
}

/// The persisted language, or `None` when the user has never picked one.
#[cfg(windows)]
pub fn saved_locale() -> Option<Locale> {
    read_pref("Language").and_then(|tag| Locale::from_tag(&tag))
}

#[cfg(windows)]
pub fn save_locale(locale: Locale) -> Result<(), String> {
    write_pref("Language", locale.tag())
}

#[cfg(windows)]
pub fn saved_theme() -> Option<Theme> {
    read_pref("Theme").as_deref().and_then(Theme::from_tag)
}

#[cfg(windows)]
pub fn save_theme(theme: Theme) -> Result<(), String> {
    write_pref("Theme", theme.tag())
}

/// `--lang` on the command line. Only the macOS front end parses it, and it
/// outranks the stored preference for that run without overwriting it.
#[cfg(not(windows))]
static LANGUAGE_OVERRIDE: std::sync::OnceLock<Option<String>> = std::sync::OnceLock::new();

#[cfg(not(windows))]
pub fn set_language_override(lang: Option<&str>) {
    let _ = LANGUAGE_OVERRIDE.set(lang.map(str::to_owned));
}

#[cfg(not(windows))]
fn cli_locale() -> Option<Locale> {
    LANGUAGE_OVERRIDE
        .get()
        .and_then(|o| o.as_deref())
        .and_then(Locale::from_tag)
}

#[cfg(windows)]
fn cli_locale() -> Option<Locale> {
    None
}

/// Pick the language from the resolution order at the top of this file.
/// `saved` is whatever the host's preference store returned.
pub fn preferred_locale(saved: Option<Locale>) -> Locale {
    cli_locale()
        .or(saved)
        .or_else(os_locale)
        .unwrap_or(Locale::DEFAULT)
}

/// The OS language, when it is one we translate.
fn os_locale() -> Option<Locale> {
    locale_from_os_tag().or_else(|| {
        ["LC_ALL", "LC_CTYPE", "LANG"]
            .iter()
            .filter_map(|name| std::env::var(name).ok())
            .find_map(|value| Locale::from_tag(&value))
    })
}

#[cfg(windows)]
fn locale_from_os_tag() -> Option<Locale> {
    use windows::Win32::Globalization::GetUserDefaultLocaleName;

    // LOCALE_NAME_MAX_LENGTH = 85
    let mut buf = [0u16; 85];
    let len = unsafe { GetUserDefaultLocaleName(&mut buf) };
    if len <= 0 {
        return None;
    }
    let end = (len as usize).saturating_sub(1);
    Locale::from_tag(&String::from_utf16_lossy(&buf[..end]))
}

/// macOS has no `LANG` when launched from Finder, which is what the shell's
/// `LC_ALL` fallback assumes. Read the same value `defaults read -g
/// AppleLocale` prints, from the global preferences domain.
#[cfg(target_os = "macos")]
fn locale_from_os_tag() -> Option<Locale> {
    use core_foundation::base::TCFType;
    use core_foundation::string::CFString;
    use core_foundation_sys::preferences::CFPreferencesCopyAppValue;

    let tag = unsafe {
        let key = CFString::new("AppleLocale");
        // The literal is the documented name of the global domain, not a typo
        // for one of the kCFPreferences* constants.
        let domain = CFString::new("kCFPreferencesGlobal");
        let value =
            CFPreferencesCopyAppValue(key.as_concrete_TypeRef(), domain.as_concrete_TypeRef());
        if value.is_null() {
            return None;
        }
        // CFPreferencesCopyAppValue follows the Create rule, so the returned
        // reference is ours to release; wrapping it hands that to Drop.
        CFString::wrap_under_create_rule(value as _).to_string()
    };
    Locale::from_tag(&tag)
}

#[cfg(not(any(windows, target_os = "macos")))]
fn locale_from_os_tag() -> Option<Locale> {
    None
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Positive control for the two loops below: a row list that silently
    /// stopped covering every field would turn them into a no-op, so pin the
    /// width and the shape of one table's rows.
    #[test]
    fn row_enumeration_covers_every_field() {
        let rows = EN.rows();
        let expected = if cfg!(windows) { 48 } else { 38 };
        assert_eq!(rows.len(), expected, "rows() drifted from the field list");
        let (names, _): (Vec<_>, Vec<_>) = rows.iter().cloned().unzip();
        assert_eq!(
            names.first().copied(),
            Some("window_title"),
            "rows() must start at the first field"
        );
        assert!(
            names.windows(2).all(|pair| pair[0] != pair[1]),
            "a field name appears twice in rows()"
        );
    }

    #[test]
    fn no_table_leaves_a_row_empty() {
        for (locale, table) in [(Locale::En, EN), (Locale::Ja, JA), (Locale::Zh, ZH)] {
            for (name, value) in table.rows() {
                assert!(
                    !value.trim().is_empty(),
                    "{locale:?} left {name} empty — the panel would render a blank label"
                );
            }
        }
    }

    #[test]
    fn translations_are_not_left_in_english() {
        // Struct literals make a *missing* row a compile error; this catches a
        // row that is present but still carrying the English text.
        let en = EN.rows();
        for (locale, table) in [(Locale::Ja, JA), (Locale::Zh, ZH)] {
            for ((name, english), (got_name, value)) in en.iter().zip(table.rows()) {
                assert_eq!(*name, got_name, "tables disagree on field order");
                if *english == value && !UNTRANSLATED_BY_DESIGN.contains(name) {
                    panic!("{locale:?} left {name} untranslated: {value:?}");
                }
            }
        }
    }

    #[test]
    fn macos_variant_changes_only_the_platform_rows() {
        for locale in Locale::ALL {
            let base = Strings::resolve(locale, Platform::Windows);
            let mac = Strings::resolve(locale, Platform::MacOS);
            for ((name, windows_value), (_, mac_value)) in base.rows().into_iter().zip(mac.rows()) {
                assert!(
                    !mac_value.trim().is_empty(),
                    "{locale:?} {name} empty on macOS"
                );
                if MACOS_SPECIFIC.contains(&name) {
                    // about_body names the toolkit and the shell, so both
                    // platforms read differently by construction; the rest of
                    // the list is a fixed set of Windows-only claims.
                    if name != "about_body" {
                        assert_ne!(
                            windows_value, mac_value,
                            "{locale:?} {name} was meant to be rewritten for Finder and was not"
                        );
                    }
                } else {
                    assert_eq!(
                        windows_value, mac_value,
                        "{locale:?} {name} differs per platform, which the shared panel does not expect"
                    );
                }
            }
        }
    }

    #[test]
    fn language_tags_resolve_to_the_right_locale() {
        assert_eq!(Locale::from_tag("zh"), Some(Locale::Zh));
        assert_eq!(Locale::from_tag("zh_CN"), Some(Locale::Zh));
        assert_eq!(Locale::from_tag("zh-Hans-CN"), Some(Locale::Zh));
        assert_eq!(Locale::from_tag("chinese"), Some(Locale::Zh));
        assert_eq!(Locale::from_tag("JA"), Some(Locale::Ja));
        assert_eq!(Locale::from_tag("en-GB"), Some(Locale::En));
        // An unknown tag must not guess: that is how a German system would
        // otherwise look like an explicit request for English.
        assert_eq!(Locale::from_tag("klingon"), None);
    }

    #[test]
    fn endonyms_are_all_distinct() {
        let names: Vec<&str> = Locale::ALL.iter().map(|l| l.endonym()).collect();
        for (a, b) in names.iter().zip(names.iter().skip(1)) {
            assert_ne!(a, b, "two languages would share one keycap label");
        }
    }

    #[test]
    fn chinese_is_the_last_resort() {
        assert_eq!(Locale::DEFAULT, Locale::Zh);
        // Nothing stored, nothing recognised from the OS.
        assert_eq!(Locale::from_tag("de-DE"), None);
    }

    #[test]
    fn theme_tags_round_trip_and_toggle() {
        assert_eq!(Theme::DEFAULT, Theme::Dark);
        for theme in [Theme::Dark, Theme::Light] {
            assert_eq!(Theme::from_tag(theme.tag()), Some(theme));
            assert_eq!(theme.toggled().toggled(), theme);
            assert_eq!(Theme::from_tag(&theme.tag().to_uppercase()), Some(theme));
        }
        assert_eq!(Theme::from_tag("system"), None);
        assert_eq!(Theme::from_tag(""), None);
    }

    #[test]
    fn locale_tags_round_trip_through_the_preference_store() {
        for locale in Locale::ALL {
            assert_eq!(Locale::from_tag(locale.tag()), Some(locale));
            // Each endonym is what that language's speakers call it, so English
            // is legitimately ASCII; what matters is that none is empty or shared.
            assert!(!locale.endonym().is_empty());
        }
    }

    #[test]
    fn theme_tags_round_trip() {
        assert_eq!(Theme::from_tag("dark"), Some(Theme::Dark));
        assert_eq!(Theme::from_tag("LIGHT"), Some(Theme::Light));
        assert_eq!(Theme::from_tag("system"), None);
        assert_eq!(Theme::DEFAULT, Theme::Dark);
        for theme in [Theme::Dark, Theme::Light] {
            assert_eq!(Theme::from_tag(theme.tag()), Some(theme));
        }
    }
}
