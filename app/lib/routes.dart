import 'package:app/screens/conversations.dart';
import 'package:go_router/go_router.dart';

GoRouter getRouter() {
  return GoRouter(
    routes: <RouteBase>[
      GoRoute(
        path: '/',
        builder: (context, state) {
          return const ConversationsScreen();
        },
      ),
    ],
  );
}
