use serde::Deserialize;

#[derive(Debug, Clone, Deserialize)]
pub struct PoolConfig {
    pub host: String,
    pub port: u16,
    pub user: String,
    pub password: String,
    pub dbname: String,
    #[serde(default = "default_max_size")]
    pub max_size: usize,
}

fn default_max_size() -> usize {
    16
}

impl PoolConfig {
    pub fn from_url(url: &str) -> crate::error::Result<Self> {
        let url = url::Url::parse(url).map_err(|e| crate::error::PoolError::Config(e.to_string()))?;
        Ok(Self {
            host: url.host_str().unwrap_or("localhost").to_string(),
            port: url.port().unwrap_or(5432),
            user: url.username().to_string(),
            password: url.password().unwrap_or("").to_string(),
            dbname: url.path().trim_start_matches('/').to_string(),
            max_size: default_max_size(),
        })
    }
}
