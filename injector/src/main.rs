use flate2::read::GzDecoder;
use glob::glob;
use sha2::{Digest, Sha256};
use std::env;
use std::fs::File;
use std::io::Read;
use std::io::Write;
use std::path::PathBuf;
use tar::Archive;

// Inspired by https://medium.com/@nlauchande/rust-coding-up-a-simple-concatenate-files-tool-and-first-impressions-a8cbe680e887

// read the binary contents of a file
fn get_file(path: &PathBuf) -> std::io::Result<Vec<u8>> {
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

// merge all given files into one buffer
fn collect_binary_data(paths: &Vec<PathBuf>) -> std::io::Result<Vec<u8>> {
    // create an empty buffer
    let mut buffer = Vec::new();

    // add contents of all files in paths to buffer
    for path in paths {
        println!("Processing {}", path.display());
        let new_content = get_file(&path);
        buffer
            .write(&new_content.unwrap())
            .expect("Could not add the file contents to the merged file buffer");
    }

    Ok(buffer)
}

fn main() {
    let args: Vec<String> = env::args().collect();

    // get the list of file matches to merge
    let file_partials: Result<Vec<_>, _> = glob("zarf-payload-*")
        .expect("Failed to read glob pattern")
        .collect();

    let mut file_partials = file_partials.unwrap();

    // ensure a default sort-order
    file_partials.sort();

    // get a buffer of the final merged file contents
    let contents = collect_binary_data(&file_partials).unwrap();

    if args.len() > 1 {
        let sha_sum = &args[1];

        // create a Sha256 object
        let mut hasher = Sha256::new();

        // write input message
        hasher.update(&contents);

        // read hash digest and consume hasher
        let result = hasher.finalize();

        assert_eq!(result[..], sha_sum.as_bytes()[..]);
    }

    let tar = GzDecoder::new(&contents[..]);
    let mut archive = Archive::new(tar);
    archive
        .unpack(".")
        .expect("Unable to unarchive the resulting tarball");
}
