import 'package:app/models/login.dart';
import 'package:app/repositories/auth_repository.dart';
import 'package:flutter/material.dart';

class LoginViewModel extends ChangeNotifier {
  LoginViewModel({required this._authRepository});

  bool isLoading = false;
  String? errorMessage;

  final AuthRepository _authRepository;

  Future<void> login(String email, String password) async {
    isLoading = true;
    errorMessage = null;
    notifyListeners();

    try {
      _authRepository.login(LoginRequest(email: email, password: password));
    } catch (e) {
      errorMessage = e.toString();
    } finally {
      isLoading = false;
      notifyListeners();
    }
  }
}
