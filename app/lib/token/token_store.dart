import 'package:flutter/material.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class TokenStore extends ChangeNotifier {
  TokenStore();

  String? _token;
  String? get token => _token;

  final _storage = const FlutterSecureStorage();

  static const _storeKey = 'access_token';

  Future<void> setToken(String value) async {
    await _storage.write(key: _storeKey, value: value);
    _token = value;
  }

  Future<void> init() async {
    String? value = await _storage.read(key: _storeKey);
    _token = value;
  }

  void invalidate() {
    _token = null;
    notifyListeners();
  }
}
