//! The panel's view model: everything the Dioxus view reads and writes, as
//! plain data with no renderer in sight.
//!
//! This state used to live half in `UiModel` (Windows) and half in
//! `StoredSettings` (macOS), while the widget tree held the only copy the user
//! was actually looking at — which forced the round-trip tests into `ui.rs`,
//! behind `#[cfg(windows)]` and a GUI test backend pinned to one thread. The
//! view model is now the single source of truth for both front ends, so the
//! same tests run on every platform and need no display at all.

use arcthumb::settings::{CoverMode, SortOrder};

/// Which of the two keycap grids a click landed in.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Grid {
    /// Shell types ArcThumb can register / filter.
    Archive,
    /// Image formats eligible as a thumbnail source inside an archive.
    Image,
}

/// One keycap: a file-type name and whether it is lit.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Row {
    pub name: String,
    pub on: bool,
}

impl Row {
    fn new(name: impl Into<String>, on: bool) -> Self {
        Self {
            name: name.into(),
            on,
        }
    }
}

/// The whole control surface, read back from the store at startup and after
/// every Apply so the panel reflects what is on disk rather than what the user
/// meant to write.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Panel {
    /// Parallel to `registry::EXTENSIONS` on Windows and
    /// `SUPPORTED_ARCHIVE_EXTS` on macOS — index order is bit order.
    pub archive: Vec<Row>,
    /// Parallel to `settings::SUPPORTED_IMAGE_EXTS`.
    pub image: Vec<Row>,
    pub sort: SortOrder,
    pub cover: CoverMode,
    pub preview: bool,
    pub border: bool,
    pub label: bool,
    pub log: bool,
}

impl Panel {
    /// Build both grids from a name list and its bitmask, bit `i` driving row
    /// `i`. The row *is* the live state for the grids: the mask is only how it
    /// crosses the store boundary.
    pub fn from_masks(names: &[&'static str], mask: u32) -> Vec<Row> {
        names
            .iter()
            .enumerate()
            .map(|(i, name)| Row::new(*name, (mask & (1u32 << i)) != 0))
            .collect()
    }

    /// Fold a grid back into its bitmask. Bits past 31 are dropped: the store
    /// is a `u32`, so a longer list than that cannot be represented and must
    /// not be allowed to alias bit 0.
    pub fn mask_of(rows: &[Row]) -> u32 {
        rows.iter().enumerate().take(32).fold(
            0u32,
            |acc, (i, row)| {
                if row.on { acc | (1u32 << i) } else { acc }
            },
        )
    }

    /// The archive grid as a fixed-size flag array, for the Windows
    /// `ShellEx` binding loop, which is written against
    /// `[bool; registry::EXTENSIONS.len()]`.
    ///
    /// Short input zero-fills rather than panicking: a stale `EXTENSIONS` list
    /// on one platform must not take the whole dialog down, and the missing
    /// The Windows representation: the `ShellEx` binding loop is written
    /// against `[bool; registry::EXTENSIONS.len()]`, so only that build needs
    /// these two.
    #[cfg(windows)]
    /// types then simply read as unregistered, which is the honest state.
    pub fn flags<const N: usize>(rows: &[Row]) -> [bool; N] {
        std::array::from_fn(|i| rows.get(i).is_some_and(|row| row.on))
    }

    /// Flip one keycap. Out-of-range indices are ignored — the view hands back
    /// an index it read from the grid, so a stale frame after a language swap
    /// is the only way to hit that path.
    pub fn toggle(&mut self, grid: Grid, index: usize) {
        let rows = match grid {
            Grid::Archive => &mut self.archive,
            Grid::Image => &mut self.image,
        };
        if let Some(row) = rows.get_mut(index) {
            row.on = !row.on;
        }
    }

    pub fn image_mask(&self) -> u32 {
        Self::mask_of(&self.image)
    }

    pub fn archive_mask(&self) -> u32 {
        Self::mask_of(&self.archive)
    }

    /// Build a grid from a parallel `bool` list, which is how the Windows side
    /// The Windows representation: the `ShellEx` binding loop is written
    /// against `[bool; registry::EXTENSIONS.len()]`, so only that build needs
    /// these two.
    #[cfg(windows)]
    /// keeps its extension state (`[bool; registry::EXTENSIONS.len()]`).
    pub fn from_flags(names: &[&'static str], flags: &[bool]) -> Vec<Row> {
        names
            .iter()
            .enumerate()
            .map(|(i, name)| Row::new(*name, flags.get(i).copied().unwrap_or(false)))
            .collect()
    }

    /// `true` when nothing the user can edit differs from the snapshot the
    /// panel loaded. Drives the Apply keycap, which is a hardware key that
    /// should look inert when there is nothing to write.
    pub fn is_dirty_vs(&self, loaded: &Panel) -> bool {
        self != loaded
    }
}

/// The sort keycap at `index`, in the order the panel draws them. Explicit
/// rather than `transmute`-adjacent arithmetic so reordering the row cannot
/// silently change which setting gets saved.
pub fn sort_from_index(index: usize) -> SortOrder {
    match index {
        1 => SortOrder::Alphabetical,
        _ => SortOrder::Natural,
    }
}

/// The cover keycap at `index`. Out-of-range reads as the default, matching
/// [`sort_from_index`].
pub fn cover_from_index(index: usize) -> CoverMode {
    match index {
        1 => CoverMode::Only,
        2 => CoverMode::Ignore,
        _ => CoverMode::Prefer,
    }
}

/// Where the panel draws the current sort order. The inverse of
/// [`sort_from_index`], kept beside it so the pair cannot drift.
pub fn sort_index(order: SortOrder) -> usize {
    match order {
        SortOrder::Natural => 0,
        SortOrder::Alphabetical => 1,
    }
}

/// The inverse of [`cover_from_index`].
pub fn cover_index(mode: CoverMode) -> usize {
    match mode {
        CoverMode::Prefer => 0,
        CoverMode::Only => 1,
        CoverMode::Ignore => 2,
    }
}

/// The panel's theme. It lives with the language preference, not the GUI layer,
/// so `settings_store` and `--lang` can name it in a build with no Dioxus.
pub use crate::locale::Theme;

/// What the LCD status strip reads out. The latin prefix (`READY` / `SAVE` /
/// `ERR`) is silk-screen and does not translate; the text after it does.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Status {
    /// Nothing has happened since the panel opened.
    Ready,
    /// The last Apply wrote, and what the store said afterwards.
    Saved,
    /// The last operation failed. Carries the reason the store gave.
    Error(String),
}

impl Status {
    pub fn silk(self: &Status) -> &'static str {
        match self {
            Status::Ready => "READY",
            Status::Saved => "SAVE",
            Status::Error(_) => "ERR",
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const NAMES: &[&str] = &[".zip", ".cbz", ".rar", ".cbr", ".7z", ".cb7"];

    fn grid(mask: u32) -> Vec<Row> {
        Panel::from_masks(NAMES, mask)
    }

    /// A panel whose two grids are the same `NAMES` list, so a test can name
    /// one mask and get a whole comparable [`Panel`].
    fn panel_with(mask: u32) -> Panel {
        Panel {
            archive: grid(mask),
            image: grid(mask),
            sort: SortOrder::Natural,
            cover: CoverMode::Prefer,
            preview: false,
            border: false,
            label: false,
            log: false,
        }
    }

    #[test]
    fn mask_to_rows_round_trips_bit_by_bit() {
        for i in 0..NAMES.len() {
            let rows = grid(1u32 << i);
            assert_eq!(
                rows.iter().filter(|row| row.on).count(),
                1,
                "exactly one keycap should light for bit {i}"
            );
            assert!(rows[i].on, "bit {i} should light row {i}");
            assert_eq!(Panel::mask_of(&rows), 1u32 << i);
        }
    }

    #[test]
    fn every_bit_and_its_complement_round_trip() {
        let all = (1u32 << NAMES.len()) - 1;
        assert_eq!(Panel::mask_of(&grid(all)), all);
        assert_eq!(Panel::mask_of(&grid(0)), 0);
        for i in 0..NAMES.len() {
            let one_off = all & !(1u32 << i);
            let rows = grid(one_off);
            assert!(!rows[i].on, "row {i} should be the cleared one");
            assert_eq!(Panel::mask_of(&rows), one_off);
        }
    }

    #[test]
    fn toggle_is_its_own_inverse_and_ignores_garbage() {
        let mut panel = panel_with(0b0010_0100);
        assert!(panel.archive[2].on);
        panel.toggle(Grid::Archive, 2);
        assert!(!panel.archive[2].on);
        panel.toggle(Grid::Archive, 2);
        assert!(panel.archive[2].on);
        // A stale index from a frame that no longer exists must not panic or
        // wrap around onto a real row.
        panel.toggle(Grid::Archive, 9_000);
        panel.toggle(Grid::Image, 9_000);
        assert_eq!(panel, panel_with(0b0010_0100));
    }

    #[cfg(windows)]
    #[test]
    fn flags_zero_fill_a_short_grid() {
        let rows = grid(0b0000_0011);
        let flags: [bool; 12] = Panel::flags(&rows);
        assert_eq!(flags[0], true);
        assert_eq!(flags[1], true);
        assert_eq!(flags[2..], [false; 10], "missing rows read as off");
    }

    #[test]
    fn mask_ignores_bits_beyond_a_u32() {
        // A 40-row list must not alias row 32 onto bit 0; the store is a u32.
        let rows: Vec<Row> = (0..40).map(|i| Row::new(format!(".x{i}"), true)).collect();
        assert_eq!(rows.len(), 40);
        assert_eq!(Panel::mask_of(&rows), u32::MAX, "only the low 32 rows fit");
        let mut tail_only: Vec<Row> = (0..40)
            .map(|i| Row::new(format!(".x{i}"), i >= 32))
            .collect();
        tail_only.truncate(40);
        assert_eq!(
            Panel::mask_of(&tail_only),
            0,
            "rows past bit 31 are dropped, not wrapped"
        );
    }

    #[test]
    fn dirtiness_tracks_only_editable_state() {
        let loaded = panel_with(0b1111_1100);
        let mut edited = loaded.clone();
        assert!(!edited.is_dirty_vs(&loaded));
        edited.toggle(Grid::Image, 0);
        assert!(edited.is_dirty_vs(&loaded));
        edited.toggle(Grid::Image, 0);
        assert!(
            !edited.is_dirty_vs(&loaded),
            "toggling back is not a change"
        );
    }

    #[test]
    fn status_prefixes_are_distinct_silk_screen_marks() {
        let marks = [
            Status::Ready.silk(),
            Status::Saved.silk(),
            Status::Error("x".into()).silk(),
        ];
        assert_eq!(marks, ["READY", "SAVE", "ERR"]);
        assert!(
            marks.windows(2).all(|pair| pair[0] != pair[1]),
            "an operator must be able to tell the three apart at a glance"
        );
    }
}
