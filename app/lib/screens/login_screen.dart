import 'package:app/viewmodels/login_vm.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class LoginScreen extends StatefulWidget {
  const LoginScreen({super.key, required this._vm});
  final LoginViewModel _vm;
  @override
  State<StatefulWidget> createState() => _LoginScreenState();
}

class _LoginScreenState extends State<LoginScreen> {
  final _emailController = TextEditingController();
  final _passwordController = TextEditingController();

  @override
  void initState() {
    super.initState();

    widget._vm.addListener(_onLoginResult);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Log in')),
      body: Padding(
        padding: const EdgeInsets.all(16.0),
        child: Center(
          child: Column(
            children: [
              TextField(
                controller: _emailController,
                decoration: InputDecoration(
                  labelText: 'Email', // TODO: add localization
                ),
              ),
              TextField(
                controller: _passwordController,
                decoration: InputDecoration(labelText: 'Password'),
                obscureText: true,
              ),
              Padding(
                padding: const EdgeInsets.only(top: 8.0),
                child: ListenableBuilder(
                  listenable: widget._vm,
                  builder: (context, child) {
                    return FilledButton(
                      onPressed: widget._vm.isLoading ? null : _onLoginPress,
                      child: widget._vm.isLoading
                          ? const CircularProgressIndicator()
                          : const Text('Log in'),
                    );
                  },
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  void _onLoginPress() {
    final email = _emailController.text;
    final password = _passwordController.text;

    widget._vm.login(email, password);
  }

  void _onLoginResult() {
    if (!mounted) return;

    final error = widget._vm.errorMessage;
    if (error != null) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text(error)));
      return;
    }

    context.go('/');
  }
}
