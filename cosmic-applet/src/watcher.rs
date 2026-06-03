use cosmic::iced::futures::channel::mpsc as futures_mpsc;
use cosmic::iced::futures::{FutureExt as _, SinkExt as _, StreamExt as _};
use cosmic::iced::stream;
use cosmic::iced::Subscription;
use notify::{RecommendedWatcher, RecursiveMode, Watcher};
use std::path::PathBuf;
use std::time::Duration;

/// Emits `()` whenever the state file may have changed.
///
/// Watches the parent directory (atomic rename changes the inode, so watching
/// the file directly would stop receiving events after the first write) and
/// filters to the target filename. Also fires on a 30 s fallback tick so the
/// UI stays consistent even if inotify misses an event.
///
/// Emits once immediately on startup so the UI can initialise from the current
/// file contents before any change arrives.
pub fn subscription(state_path: PathBuf) -> Subscription<()> {
    Subscription::run_with(state_path, |state_path: &PathBuf| {
        let state_path = state_path.clone();
        stream::channel(8, move |mut output: futures_mpsc::Sender<()>| async move {
            let dir = state_path
                .parent()
                .map(|p| p.to_path_buf())
                .unwrap_or_else(|| PathBuf::from("/tmp"));
            let filename = state_path
                .file_name()
                .map(|n: &std::ffi::OsStr| n.to_os_string());

            // Channel used to bridge the synchronous notify callback into async.
            let (notify_tx, mut notify_rx) = futures_mpsc::unbounded::<()>();

            let filename_cb = filename.clone();
            let _watcher: Option<RecommendedWatcher> = match RecommendedWatcher::new(
                move |res: notify::Result<notify::Event>| {
                    if let Ok(event) = res {
                        let hit = event.paths.iter().any(|p| {
                            p.file_name()
                                .map(|n| Some(n.to_os_string()) == filename_cb)
                                .unwrap_or(false)
                        });
                        if hit {
                            let _ = notify_tx.unbounded_send(());
                        }
                    }
                },
                notify::Config::default(),
            ) {
                Ok(mut w) => {
                    let _ = w.watch(&dir, RecursiveMode::NonRecursive);
                    Some(w)
                }
                Err(_) => None,
            };

            // Emit immediately so the UI initialises from the current file.
            let _ = output.send(()).await;

            loop {
                // Wait for either an inotify event or the 30 s fallback timer.
                cosmic::iced::futures::select! {
                    msg = notify_rx.next().fuse() => {
                        if msg.is_none() {
                            // Sender dropped — shouldn't happen, but bail gracefully.
                            break;
                        }
                    }
                    _ = tokio::time::sleep(Duration::from_secs(30)).fuse() => {}
                }

                if output.send(()).await.is_err() {
                    break;
                }
            }

            std::future::pending::<()>().await
        })
    })
}
