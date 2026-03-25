fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_build::configure()
        .build_server(true)
        .build_client(false)
        .compile_protos(
            &["../../../proto/engine/v1/engine.proto"],
            &["../../../proto"],
        )?;
    Ok(())
}
