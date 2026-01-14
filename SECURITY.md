# Security Policy

## Supported Versions

We release patches for security vulnerabilities in the latest released version of jsonrpc4go. We recommend users keep their installations up-to-date to ensure they have the latest security patches.

## Reporting a Vulnerability

If you discover a security vulnerability in jsonrpc4go, please report it responsibly by contacting us directly via email to the maintainers instead of creating a public GitHub issue. This helps us protect users by coordinating the fix and disclosure properly.

Please include the following details in your report:
- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Your suggested remediation (if any)

Our team will acknowledge receipt of your report within 48 hours and provide updates on the timeline for addressing the issue.

## Security Best Practices

When using jsonrpc4go, consider the following security measures:
- Validate and sanitize all inputs before passing them to RPC methods
- Implement proper authentication and authorization mechanisms
- Use HTTPS/TLS for all RPC communications when possible
- Apply rate limiting to prevent abuse
- Regularly update to the latest version to receive security patches
- Monitor logs for suspicious activities

## Known Security Considerations

- The library allows dynamic method registration which could lead to unexpected method exposure if not carefully managed
- Input validation is the responsibility of the individual RPC method implementations
- Users should be aware of the security implications of exposing services over the network