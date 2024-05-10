// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

use std::env;
use std::fs;
use std::fs::File;
use std::io;
use std::io::Read;
use std::io::Write;
use std::path::PathBuf;

use regex_lite::Regex;
use tokio_util::io::ReaderStream;
use flate2::read::GzDecoder;
use glob::glob;
use hex::ToHex;
use serde_json::Value;
use sha2::{Digest, Sha256};
use tar::Archive;
use axum::{
    routing::get, 
    Router, 
    extract::Path, 
    http::StatusCode,
    response::{Response, IntoResponse},
    body::Body,
};
const OCI_MIME_TYPE: &str = "application/vnd.oci.image.manifest.v1+json";

// Reads the binary contents of a file
fn get_file(path: &PathBuf) -> io::Result<Vec<u8>> {
    // open the file
    let mut f = File::open(path)?;
    // create an empty buffer
    let mut buffer = Vec::new();

    // read the whole file
    match f.read_to_end(&mut buffer) {
        Ok(_) => Ok(buffer),
        Err(e) => Err(e),
    }
}


// Merges all given files into one buffer
fn collect_binary_data(paths: &Vec<PathBuf>) -> io::Result<Vec<u8>> {
    // create an empty buffer
    let mut buffer = Vec::new();

    // add contents of all files in paths to buffer
    for path in paths {
        println!("Processing {}", path.display());
        let new_content = get_file(path);
        buffer
            .write(&new_content.unwrap())
            .expect("Could not add the file contents to the merged file buffer");
    }

    Ok(buffer)
}

/// Unpacks the zarf-payload-* configmaps back into a tarball, then unpacks into the CWD
///
/// Inspired by https://medium.com/@nlauchande/rust-coding-up-a-simple-concatenate-files-tool-and-first-impressions-a8cbe680e887
fn unpack(sha_sum: &String) {
    // get the list of file matches to merge
    let file_partials: Result<Vec<_>, _> = glob("zarf-payload-*")
        .expect("Failed to read glob pattern")
        .collect();

    let mut file_partials = file_partials.unwrap();

    // ensure a default sort-order
    file_partials.sort();

    // get a buffer of the final merged file contents
    let contents = collect_binary_data(&file_partials).unwrap();

    // create a Sha256 object
    let mut hasher = Sha256::new();

    // write input message
    hasher.update(&contents);

    // read hash digest and consume hasher
    let result = hasher.finalize();
    let result_string = result.encode_hex::<String>();
    assert_eq!(*sha_sum, result_string);

    // write the merged file to disk and extract it
    let tar = GzDecoder::new(&contents[..]);
    let mut archive = Archive::new(tar);
    archive
        .unpack("/zarf-seed")
        .expect("Unable to unarchive the resulting tarball");
}



/// Starts a static docker compliant registry server that only serves the single image from the CWD
///
/// (which is a OCI image layout):
///
/// index.json - the image index
/// blobs/sha256/<sha256sum> - the image layers
/// oci-layout - the OCI image layout
fn start_seed_registry() -> Router{
    // The name and reference parameter identify the image
    // The reference may include a tag or digest.
    Router::new()
    .route("/v2/*path", get(handler))
    .route("/v2/", get(|| async { 
    Response::builder()
        .status(StatusCode::OK)
        .header("Content-Type", "application/json; charset=utf-8")
        .header("Docker-Distribution-Api-Version", "registry/2.0")
        .header("X-Content-Type-Options", "nosniff")
        .body(Body::empty())
        .unwrap()
    }))
    .route("/v2", get(|| async { 
        Response::builder()
            .status(StatusCode::OK)
            .header("Content-Type", "application/json; charset=utf-8")
            .header("Docker-Distribution-Api-Version", "registry/2.0")
            .header("X-Content-Type-Options", "nosniff")
            .body(Body::empty())
            .unwrap()
        }))
}

async fn handler(Path(path): Path<String>) -> Response {
    println!("request: {}", path);
    let path = &path;
    let manifest = Regex::new("(.+)/manifests/(.+)").unwrap();
    let blob = Regex::new(".+/([^/]+)").unwrap();

    if manifest.is_match(path){
        let caps = manifest.captures(path).unwrap();
        let name = caps.get(1).unwrap().as_str().to_string();
        let reference = caps.get(2).unwrap().as_str().to_string();
        handle_get_manifest(name, reference).await


    }else if blob.is_match(&path) {
        let caps = blob.captures(path).unwrap();
        let tag = caps.get(1).unwrap().as_str().to_string();
        handle_get_digest(tag).await
    } else {
        Response::builder()
        .status(StatusCode::NOT_FOUND)
        .body(format!("Not Found"))
        .unwrap()
        .into_response()
  }
}

/// Handles the GET request for the manifest (only returns a OCI manifest regardless of Accept header)
async fn handle_get_manifest(name: String, reference: String) -> Response {
    println!("name {}, reference {}", name, reference);
    let index = fs::read_to_string(PathBuf::from("/zarf-seed").join("index.json")).expect("read index.json");
    let json: Value = serde_json::from_str(&index).expect("unable to parse index.json");
    
    let mut sha_manifest: String = "".to_owned();

    if reference.starts_with("sha256:") {
        sha_manifest = reference.strip_prefix("sha256:").unwrap().to_owned();
    } else {
        for manifest in json["manifests"].as_array().unwrap() {
            let image_base_name = manifest["annotations"]["org.opencontainers.image.base.name"]
                .as_str()
                .unwrap();
            let requested_reference = name.to_owned() + ":" + &reference;
            if requested_reference == image_base_name {
                sha_manifest = manifest["digest"]
                    .as_str()
                    .unwrap()
                    .strip_prefix("sha256:")
                    .unwrap()
                    .to_owned();
            }
        }
    }
    if !sha_manifest.is_empty() {
        let file_path = PathBuf::from("/zarf-seed").to_owned().join( "blobs").join( &sha_manifest);
        match tokio::fs::File::open(&file_path).await {
            Ok(file) => {
                let stream = ReaderStream::new(file);
                Response::builder()
                    .status(StatusCode::OK)
                    .header("Content-Type", OCI_MIME_TYPE)
                    .header("Docker-Content-Digest", sha_manifest.clone())
                    .header("Etag", format!("sha256:{}", sha_manifest))
                    .header("Docker-Distribution-Api-Version", "registry/2.0")
                    .body(Body::from_stream(stream))
                    .unwrap()
            }
            Err(err) => 
            Response::builder()
            .status(StatusCode::NOT_FOUND)
            .body(format!("File not found: {}", err))
            .unwrap()
            .into_response()
            }
    }else {
    Response::builder()
    .status(StatusCode::NOT_FOUND)
    .body(format!("Not Found"))
    .unwrap()

    .into_response()
    }
}


/// Handles the GET request for a blob
async fn handle_get_digest(tag: String) -> Response {
    let blob_root = PathBuf::from("/zarf-seed").join("blobs").join("sha256");
    let path = blob_root.join(tag.strip_prefix("sha256:").unwrap());

    let data = fs::read_to_string(path).expect("read index.json");

    Response::builder()
        .status(StatusCode::OK)
        .header("Content-Type", "application/octet-stream")
        .header("Docker-Content-Digest", tag.to_owned())
        .header("Etag", tag.to_owned())
        .header("Docker-Distribution-Api-Version", "registry/2.0")
        .header("Cache-Control", "max-age=31536000")
        .body(Body::from(data))
        .unwrap()
}

#[tokio::main(flavor = "current_thread")]
async fn main() {
    let args: Vec<String> = env::args().collect();

    println!("unpacking: {}", args[1]);
    let payload_sha = &args[1];

    unpack(payload_sha);

    let listener = tokio::net::TcpListener::bind("0.0.0.0:5000")
    .await
    .unwrap();
    println!("listening on {}", listener.local_addr().unwrap());
    axum::serve(listener, start_seed_registry())
        .await
        .unwrap();
    println!("Usage: {} <sha256sum>", args[1]);

}