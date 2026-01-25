//! Request Signing Module
//!
//! Provides HMAC-SHA256 signing for agent-to-backend requests
//! to prevent tampering and replay attacks.

use hmac::{Hmac, Mac};
use sha2::Sha256;

type HmacSha256 = Hmac<Sha256>;

/// Signs a request body with HMAC-SHA256
/// 
/// # Arguments
/// * `secret` - The API key or shared secret
/// * `timestamp` - Unix timestamp in seconds
/// * `body` - The request body bytes
/// 
/// # Returns
/// Hex-encoded HMAC signature
pub fn sign_request(secret: &str, timestamp: i64, body: &[u8]) -> String {
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes())
        .expect("HMAC can take key of any size");
    
    // Include timestamp in signature to prevent replay attacks
    mac.update(&timestamp.to_le_bytes());
    mac.update(body);
    
    hex::encode(mac.finalize().into_bytes())
}

/// Verifies a request signature
/// 
/// # Arguments
/// * `secret` - The API key or shared secret
/// * `timestamp` - Unix timestamp from request header
/// * `body` - The request body bytes
/// * `signature` - The signature to verify (hex-encoded)
/// 
/// # Returns
/// true if signature is valid
pub fn verify_signature(secret: &str, timestamp: i64, body: &[u8], signature: &str) -> bool {
    let expected = sign_request(secret, timestamp, body);
    // Use constant-time comparison to prevent timing attacks
    constant_time_eq(expected.as_bytes(), signature.as_bytes())
}

/// Constant-time byte comparison to prevent timing attacks
fn constant_time_eq(a: &[u8], b: &[u8]) -> bool {
    if a.len() != b.len() {
        return false;
    }
    
    let mut result = 0u8;
    for (x, y) in a.iter().zip(b.iter()) {
        result |= x ^ y;
    }
    result == 0
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sign_and_verify() {
        let secret = "sk_test_123456";
        let timestamp = 1706178000i64;
        let body = b"test request body";
        
        let signature = sign_request(secret, timestamp, body);
        assert!(verify_signature(secret, timestamp, body, &signature));
    }

    #[test]
    fn test_invalid_signature() {
        let secret = "sk_test_123456";
        let timestamp = 1706178000i64;
        let body = b"test request body";
        
        let signature = sign_request(secret, timestamp, body);
        // Tampered body should fail
        assert!(!verify_signature(secret, timestamp, b"tampered body", &signature));
        // Different secret should fail
        assert!(!verify_signature("wrong_secret", timestamp, body, &signature));
        // Different timestamp should fail
        assert!(!verify_signature(secret, timestamp + 1, body, &signature));
    }

    #[test]
    fn test_constant_time_eq() {
        assert!(constant_time_eq(b"hello", b"hello"));
        assert!(!constant_time_eq(b"hello", b"world"));
        assert!(!constant_time_eq(b"hello", b"hell"));
    }
}
