import 'dart:async';

import 'package:app/models/login.dart';
import 'package:app/token/token_store.dart';
import 'package:dio/dio.dart';

class AuthRepository {
  const AuthRepository({required this._dio, required this._tokenStore});

  final Dio _dio;
  final TokenStore _tokenStore;

  Future<void> login(LoginRequest reqData) async {
    final response = await _dio.post('/login', data: reqData.toJson());

    final respData = LoginResponse.fromJson(response.data);
    _tokenStore.setToken(respData.accessToken);
  }
}
