import 'dart:async';

import 'package:app/config.dart';
import 'package:app/repositories/auth_repository.dart';
import 'package:app/screens/conversations.dart';
import 'package:app/screens/login_screen.dart';
import 'package:app/token/token_interceptor.dart';
import 'package:app/token/token_store.dart';
import 'package:app/viewmodels/login_vm.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:talker_flutter/talker_flutter.dart';

void main() {
  final logger = TalkerFlutter.init();

  runZonedGuarded(
    () async {
      final config = Config();

      final tokenStore = TokenStore();
      await tokenStore.init();

      final tokenInterceptor = TokenInterceptor(tokenStore);

      final dio = Dio(
        BaseOptions(
          baseUrl: config.apiUrl,
          connectTimeout: Duration(seconds: 5),
          receiveTimeout: Duration(seconds: 3),
        ),
      );
      dio.interceptors.add(tokenInterceptor);
      dio.interceptors.add(LogInterceptor());

      final authRepository = AuthRepository(dio: dio, tokenStore: tokenStore);

      final loginVM = LoginViewModel(authRepository: authRepository);

      final router = GoRouter(
        initialLocation: tokenStore.token == null ? '/login' : '/',
        refreshListenable: tokenStore,
        redirect: (context, state) {
          if (tokenStore.token == null) {
            return '/login';
          }
        },
        routes: <RouteBase>[
          GoRoute(
            path: '/',
            builder: (context, state) => const ConversationsScreen(),
          ),
          GoRoute(
            path: '/login',
            builder: (context, state) => LoginScreen(vm: loginVM),
          ),
        ],
      );

      runApp(MyApp(router: router));
    },
    (Object error, StackTrace stack) {
      logger.handle(error, stack, 'Uncaught app exception');
    },
  );
}

class MyApp extends StatelessWidget {
  const MyApp({super.key, required this._router});

  final GoRouter _router;

  @override
  Widget build(BuildContext context) {
    return MaterialApp.router(
      routerConfig: _router,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.blueGrey),
      ),
    );
  }
}
