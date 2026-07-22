class ConfigException implements Exception {
  const ConfigException({required this._cause});
  final String _cause;

  @override
  String toString() => 'ConfigException: $_cause';
}

class Config {
  const Config._({required this.apiUrl});

  final String apiUrl;

  factory Config() {
    const apiUrl = String.fromEnvironment('API_URL');
    if (apiUrl.isEmpty) {
      throw const ConfigException(cause: 'API_URL is required');
    }

    return const Config._(apiUrl: apiUrl);
  }
}
