//! Background update check driver.
//!
//! Spawns a worker thread on panel startup that hits the GitHub releases API and
//! sends back a [`Prompt`]. The thread never touches the UI: the panel's runtime
//! owns the only reader of the channel, because writing a signal from another
//! thread would reach into Dioxus off-thread. `app::Host::start_prompts` hands
//! the sender over.
//!
//! The donation prompt is sent synchronously — `should_show_donation` is a
//! registry read, not a network call — so it arrives with the first frame
//! rather than one event-loop tick after it.
//!
//! If the user has opted out, the throttle window has not elapsed, no newer
//! release exists, or they already skipped this version, the thread sends
//! [`Prompt::CheckFinished`] and exits, which is the only thing that stops the
//! strip's indicator blinking.

use futures_channel::mpsc::UnboundedSender;

use crate::app::host::Prompt;
use crate::update;

/// Kick off the startup prompts. Returns immediately.
pub fn start_prompts(tx: UnboundedSender<Prompt>) {
    if let Some(version) = update::should_show_donation() {
        let _ = tx.unbounded_send(Prompt::Donation { version });
    }

    std::thread::spawn(move || {
        if !update::should_check_now() {
            // The indicator was lit for this call, so it has to be put out even
            // when the throttle means no request is made.
            let _ = tx.unbounded_send(Prompt::CheckFinished);
            return;
        }
        let Some(info) = update::check_for_update() else {
            let _ = tx.unbounded_send(Prompt::CheckFinished);
            return;
        };
        if update::is_version_skipped(&info.latest_version) {
            let _ = tx.unbounded_send(Prompt::CheckFinished);
            return;
        }
        let _ = tx.unbounded_send(Prompt::Update {
            latest: info.latest_version,
            current: update::current_version().to_string(),
            url: info.release_url,
        });
    });
}
