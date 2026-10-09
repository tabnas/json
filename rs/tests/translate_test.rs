// The translation part: what the manifest says and what the crate embeds
// are the same file.
//
// A packaged crate holds nothing outside `rs/`, so the crate embeds its
// own copy of `tabnas.plugin.json`, `rs/translate/manifest.json`, as
// `manifest_text()`. The copy is the only text a host sees, so it must be
// the file: this holds the embedded manifest to the repository's, as it
// would an embed the manifest named. Change the manifest at the root and
// run `npm run embed` in `ts/`, which copies it into `rs/translate/`; this
// fails until both are the same. JSON's render is the `json` render
// alchemy carries, not a file of this repository's, so there is no render
// to hold and no `render_text()`.

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
        "rs/translate/manifest.json is not tabnas.plugin.json: run npm run embed in ts"
    );
}

#[test]
fn the_structural_interface_names_the_builtin_render_entry() {
    let parts = tabnas_json::translate().expect("JSON carries translation parts");
    assert_eq!(parts.manifest, tabnas_json::manifest_text());
    assert_eq!(parts.lift, None);
    let render = parts.render.expect("JSON carries a render");
    assert_eq!(render.entry, "json");
    assert_eq!(render.source, None);
}

/// An embed takes a plain tree into a format's own schema. JSON's events
/// carry a plain tree, so its manifest names none and the crate carries
/// none; a manifest that named one would be held to its file here.
#[test]
fn the_embed_the_manifest_names_is_the_one_the_crate_embeds() {
    let translate = translate();
    let parts = tabnas_json::translate().expect("JSON carries translation parts");
    let Some(path) = translate.get("embed").and_then(Value::as_str) else {
        assert_eq!(
            parts.embed, None,
            "the manifest names no embed, and the crate carries one"
        );
        return;
    };
    let on_disk = fs::read_to_string(repo_root().join(path))
        .unwrap_or_else(|e| panic!("translate.embed names {path}, which cannot be read: {e}"));
    let embed = parts
        .embed
        .unwrap_or_else(|| panic!("translate.embed names {path}, and the crate carries no embed"));
    assert_eq!(embed.entry, "json-embed");
    assert_eq!(
        embed.source,
        Some(on_disk.as_str()),
        "translate.embed names {path}, and the crate embeds another text: run npm run embed in ts"
    );
}

/// JSON is read as a tree and written from one, at any root, through the
/// `json` render alchemy carries. Its events carry the tree already, so
/// there is no lift, and no render file of its own.
#[test]
fn json_reads_and_writes_a_tree_at_any_root_through_the_json_render() {
    let translate = translate();
    assert_eq!(translate["reads"], "tree");
    assert_eq!(translate["writes"], "tree");
    assert_eq!(translate["root"], "any");
    assert_eq!(translate["render"], "json");
    assert_eq!(translate.get("lift"), None);
}

/// The `json` render keeps every value and every number's spelling, bar
/// one: JSON has no spelling for a number that is not finite, and a host
/// runs the render under the convention that writes one as null. That is
/// the one loss, and the host prints it verbatim, so it is a sentence.
#[test]
fn the_loss_is_the_one_sentence_a_number_that_is_not_finite_needs() {
    let translate = translate();
    let loss = translate["loss"]
        .as_array()
        .expect("translate.loss is a list");
    assert_eq!(
        loss,
        &vec![Value::from(
            "JSON has no spelling for Infinity or NaN, so a number that is not finite is written as null."
        )]
    );
    for line in loss {
        let line = line.as_str().expect("each loss line is a string");
        assert!(
            line.starts_with(char::is_uppercase) && line.ends_with('.'),
            "{line:?} is not a sentence"
        );
    }
}
