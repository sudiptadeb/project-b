// The WebView adapter's way to the Go core. Go's engine calls arrive through the same
// PBEngineCalls table the content adapter fills, and leave as methods on the Java Engine.
// Java's calls arrive as JNI functions and leave as Go's exports.
//
// Threading: everything runs on Android's main thread, except post_wakeup.

#include <jni.h>
#include <stdlib.h>
#include <string.h>

#include "libcore.h"

static JavaVM* g_vm;
static jobject g_engine;
static jclass g_string;
static jmethodID g_string_new;
static jmethodID g_string_get_bytes;
static jstring g_utf8;

static jmethodID g_create_tab;
static jmethodID g_load_url;
static jmethodID g_go_back;
static jmethodID g_go_forward;
static jmethodID g_reload;
static jmethodID g_set_page_bounds;
static jmethodID g_set_shell_regions;
static jmethodID g_focus_page;
static jmethodID g_send_to_shell;
static jmethodID g_post_wakeup;

static JNIEnv* current_env(void) {
  JNIEnv* env;
  (*g_vm)->GetEnv(g_vm, (void**)&env, JNI_VERSION_1_6);
  return env;
}

// JNI's own string calls use modified UTF-8, which turns an emoji in a page title into
// something Go can't read, so strings cross as real UTF-8 bytes.
static jstring java_string(JNIEnv* env, const char* utf8, size_t length) {
  jbyteArray bytes = (*env)->NewByteArray(env, (jsize)length);
  (*env)->SetByteArrayRegion(env, bytes, 0, (jsize)length, (const jbyte*)utf8);
  jstring s = (jstring)(*env)->NewObject(env, g_string, g_string_new, bytes, g_utf8);
  (*env)->DeleteLocalRef(env, bytes);
  return s;
}

// The caller frees the result.
static char* c_string(JNIEnv* env, jstring s, size_t* length) {
  jbyteArray bytes = (jbyteArray)(*env)->CallObjectMethod(env, s, g_string_get_bytes, g_utf8);
  jsize n = (*env)->GetArrayLength(env, bytes);
  char* out = malloc((size_t)n + 1);
  (*env)->GetByteArrayRegion(env, bytes, 0, n, (jbyte*)out);
  out[n] = 0;
  (*env)->DeleteLocalRef(env, bytes);
  if (length) {
    *length = (size_t)n;
  }
  return out;
}

static int64_t create_tab(void) {
  JNIEnv* env = current_env();
  return (*env)->CallLongMethod(env, g_engine, g_create_tab);
}

static void load_url(int64_t tab, const char* url) {
  JNIEnv* env = current_env();
  jstring s = java_string(env, url, strlen(url));
  (*env)->CallVoidMethod(env, g_engine, g_load_url, (jlong)tab, s);
  (*env)->DeleteLocalRef(env, s);
}

static void go_back(int64_t tab) {
  JNIEnv* env = current_env();
  (*env)->CallVoidMethod(env, g_engine, g_go_back, (jlong)tab);
}

static void go_forward(int64_t tab) {
  JNIEnv* env = current_env();
  (*env)->CallVoidMethod(env, g_engine, g_go_forward, (jlong)tab);
}

static void reload(int64_t tab) {
  JNIEnv* env = current_env();
  (*env)->CallVoidMethod(env, g_engine, g_reload, (jlong)tab);
}

static void set_page_bounds(double x, double y, double w, double h, double view_w, double view_h) {
  JNIEnv* env = current_env();
  (*env)->CallVoidMethod(env, g_engine, g_set_page_bounds, x, y, w, h, view_w, view_h);
}

static void set_shell_regions(const double* rects, size_t count) {
  JNIEnv* env = current_env();
  jdoubleArray a = (*env)->NewDoubleArray(env, (jsize)(count * 4));
  (*env)->SetDoubleArrayRegion(env, a, 0, (jsize)(count * 4), rects);
  (*env)->CallVoidMethod(env, g_engine, g_set_shell_regions, a);
  (*env)->DeleteLocalRef(env, a);
}

static void focus_page(void) {
  JNIEnv* env = current_env();
  (*env)->CallVoidMethod(env, g_engine, g_focus_page);
}

static void send_to_shell(const char* json, size_t length) {
  JNIEnv* env = current_env();
  jstring s = java_string(env, json, length);
  (*env)->CallVoidMethod(env, g_engine, g_send_to_shell, s);
  (*env)->DeleteLocalRef(env, s);
}

// Any goroutine's thread lands here, and the VM doesn't know those threads.
static void post_wakeup(void) {
  JNIEnv* env;
  int known = (*g_vm)->GetEnv(g_vm, (void**)&env, JNI_VERSION_1_6) == JNI_OK;
  if (!known) {
    (*g_vm)->AttachCurrentThread(g_vm, &env, NULL);
  }
  (*env)->CallVoidMethod(env, g_engine, g_post_wakeup);
  if (!known) {
    (*g_vm)->DetachCurrentThread(g_vm);
  }
}

static const PBEngineCalls g_calls = {
    .create_tab = create_tab,
    .load_url = load_url,
    .go_back = go_back,
    .go_forward = go_forward,
    .reload = reload,
    .set_page_bounds = set_page_bounds,
    .set_shell_regions = set_shell_regions,
    .focus_page = focus_page,
    .send_to_shell = send_to_shell,
    .post_wakeup = post_wakeup,
};

// A thread Go starts can't find our classes, so everything is looked up here, on the
// thread that loaded the library.
JNIEXPORT jint JNI_OnLoad(JavaVM* vm, void* reserved) {
  g_vm = vm;
  JNIEnv* env = current_env();

  g_string = (jclass)(*env)->NewGlobalRef(env, (*env)->FindClass(env, "java/lang/String"));
  g_string_new = (*env)->GetMethodID(env, g_string, "<init>", "([BLjava/lang/String;)V");
  g_string_get_bytes = (*env)->GetMethodID(env, g_string, "getBytes", "(Ljava/lang/String;)[B");
  g_utf8 = (jstring)(*env)->NewGlobalRef(env, (*env)->NewStringUTF(env, "UTF-8"));

  jclass engine = (*env)->FindClass(env, "dev/projectb/webview/Engine");
  g_create_tab = (*env)->GetMethodID(env, engine, "createTab", "()J");
  g_load_url = (*env)->GetMethodID(env, engine, "loadUrl", "(JLjava/lang/String;)V");
  g_go_back = (*env)->GetMethodID(env, engine, "goBack", "(J)V");
  g_go_forward = (*env)->GetMethodID(env, engine, "goForward", "(J)V");
  g_reload = (*env)->GetMethodID(env, engine, "reload", "(J)V");
  g_set_page_bounds = (*env)->GetMethodID(env, engine, "setPageBounds", "(DDDDDD)V");
  g_set_shell_regions = (*env)->GetMethodID(env, engine, "setShellRegions", "([D)V");
  g_focus_page = (*env)->GetMethodID(env, engine, "focusPage", "()V");
  g_send_to_shell = (*env)->GetMethodID(env, engine, "sendToShell", "(Ljava/lang/String;)V");
  g_post_wakeup = (*env)->GetMethodID(env, engine, "postWakeup", "()V");
  return JNI_VERSION_1_6;
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_start(JNIEnv* env,
                                                            jclass cls,
                                                            jobject engine,
                                                            jstring profile_dir,
                                                            jstring blocked_hosts) {
  g_engine = (*env)->NewGlobalRef(env, engine);
  char* dir = c_string(env, profile_dir, NULL);
  char* blocked = c_string(env, blocked_hosts, NULL);
  PBStart((PBEngineCalls*)&g_calls, dir, blocked);
  free(dir);
  free(blocked);
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_stop(JNIEnv* env, jclass cls) {
  PBStop();
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_runPosted(JNIEnv* env, jclass cls) {
  PBRunPosted();
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_shellMessage(JNIEnv* env,
                                                                   jclass cls,
                                                                   jstring json) {
  size_t length;
  char* s = c_string(env, json, &length);
  PBShellMessage(s, length);
  free(s);
}

JNIEXPORT jboolean JNICALL Java_dev_projectb_webview_Core_navigationStarting(JNIEnv* env,
                                                                             jclass cls,
                                                                             jlong tab,
                                                                             jstring url) {
  char* s = c_string(env, url, NULL);
  int allowed = PBNavigationStarting(tab, s);
  free(s);
  return allowed ? JNI_TRUE : JNI_FALSE;
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_navigationFinished(JNIEnv* env,
                                                                         jclass cls,
                                                                         jlong tab,
                                                                         jstring url) {
  char* s = c_string(env, url, NULL);
  PBNavigationFinished(tab, s);
  free(s);
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_navigationFailed(JNIEnv* env,
                                                                       jclass cls,
                                                                       jlong tab,
                                                                       jstring url,
                                                                       jstring reason) {
  char* u = c_string(env, url, NULL);
  char* r = c_string(env, reason, NULL);
  PBNavigationFailed(tab, u, r);
  free(u);
  free(r);
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_historyChanged(JNIEnv* env,
                                                                     jclass cls,
                                                                     jlong tab,
                                                                     jboolean can_go_back,
                                                                     jboolean can_go_forward) {
  PBHistoryChanged(tab, can_go_back, can_go_forward);
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_titleChanged(JNIEnv* env,
                                                                   jclass cls,
                                                                   jlong tab,
                                                                   jstring title) {
  char* s = c_string(env, title, NULL);
  PBTitleChanged(tab, s);
  free(s);
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_loadingChanged(JNIEnv* env,
                                                                     jclass cls,
                                                                     jlong tab,
                                                                     jboolean loading) {
  PBLoadingChanged(tab, loading);
}

JNIEXPORT void JNICALL Java_dev_projectb_webview_Core_pageCrashed(JNIEnv* env,
                                                                  jclass cls,
                                                                  jlong tab) {
  PBPageCrashed(tab);
}
