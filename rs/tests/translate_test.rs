// The translation part: what the manifest says and what the crate embeds
// are the same file.
//
// A packaged crate holds nothing outside `rs/`, so the crate embeds its
// own copy of `tabnas.plugin.json`, `rs/translate/manifest.json`, as
// `manifest_text()`. The copy is the only text a host sees, so it must be
// the file: this holds the embedded manifest to the repository's. Change
// the manifest at the root and copy it into `rs/translate/`; this fails
// until both are the same. JSON's render is the `json` render alchemy
// carries, not a file of this repository's, so there is no render to hold
// and no `render_text()`.

use std::fs;
use std::path::Path;

use serde_json::Value;

/// The repository root: the parent of `rs/`.
fn repo_root() -> &'static Path {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .parent()
        .expect("rs/ has a parent")
}

fn translate() -> Value {
    let manifest: Value =
        serde_json::from_str(tabnas_json::manifest_text()).expect("the manifest is JSON");
    manifest
        .get("translate")
        .cloned()
        .expect("the manifest carries a translate object")
}

#[test]
fn the_manifest_the_crate_embeds_is_the_repositorys() {
    let on_disk = fs::read_to_string(repo_root().join("tabnas.plugin.json"))
        .expect("the repository has its manifest");
    assert_eq!(
        on_disk,
        tabnas_json::manifest_text(),
        "rs/translate/manifest.json is not tabnas.plugin.json: copy the manifest into rs/translate"
    );
}

/// JSON is read as a tree and written from one, through the `json` render
/// alchemy carries. Its events carry the tree already, so there is no
/// lift, and no render file of its own. It declares no loss either: that
/// render keeps every value and every number's spelling, so nothing a
/// JSON document holds is lost in writing one.
#[test]
fn json_reads_and_writes_a_tree_through_the_json_render_and_loses_nothing() {
    let translate = translate();
    assert_eq!(translate["reads"], "tree");
    assert_eq!(translate["writes"], "tree");
    assert_eq!(translate["render"], "json");
    assert_eq!(translate.get("lift"), None);
    assert_eq!(translate.get("loss"), None);
}
