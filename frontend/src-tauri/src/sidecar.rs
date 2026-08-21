use std::process::{Child, Command};
pub struct DaemonSidecar { child: Option<Child> }
impl DaemonSidecar {
  pub fn spawn() -> Self {
    let child = Command::new("../daemon/myserverd").arg("--port").arg("4096").spawn().ok();
    Self { child }
  }
}
impl Drop for DaemonSidecar { fn drop(&mut self){ if let Some(mut c)=self.child.take(){ let _=c.kill(); } } }
