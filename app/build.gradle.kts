@file:Suppress("UNUSED_EXPRESSION")

import com.android.build.api.dsl.Packaging

@Suppress("DSL_SCOPE_VIOLATION") // TODO: Remove once KTIJ-19369 is fixed
plugins {
    alias(libs.plugins.androidApplication)
    alias(libs.plugins.kotlinAndroid)
}

android {
    namespace = "io.github.asutorufa.tailscaled"
    compileSdk = 34

    defaultConfig {
        applicationId = "io.github.asutorufa.tailscaled"
        minSdk = 24
        targetSdk = 34
        versionCode = 1
        versionName = "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    splits {
        abi {
            isEnable = true

            // Resets the list of ABIs that Gradle should create APKs for to none.
            reset()

            include("x86_64", "arm64-v8a")
        }
    }

    signingConfigs {
        if (System.getenv("KEYSTORE_PATH") != null)
            create("releaseConfig") {
                storeFile = file(System.getenv("KEYSTORE_PATH"))
                keyAlias = System.getenv("KEY_ALIAS")
                storePassword = System.getenv("KEYSTORE_PASSWORD")
                keyPassword = System.getenv("KEY_PASSWORD")
            }
    }

    this.buildOutputs.all {
        val variantOutputImpl = this as com.android.build.gradle.internal.api.BaseVariantOutputImpl
        val variantName: String = variantOutputImpl.name
        variantOutputImpl.outputFileName = "tailscaled-${variantName}.apk"
    }

    buildTypes {
        release {
            // Enables code shrinking, obfuscation, and optimization for only
            // your project's release build type.
            isMinifyEnabled = true

            // Enables resource shrinking, which is performed by the
            // Android Gradle plugin.
            isShrinkResources = true

            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro"
            )


            if (System.getenv("KEYSTORE_PATH") != null)
                signingConfig = signingConfigs.getByName("releaseConfig")
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }
    fun Packaging.() {
        jniLibs {
            useLegacyPackaging = true
        }
    }
    kotlinOptions {
        jvmTarget = "1.8"
    }
    buildFeatures {
        viewBinding = true
    }
}

dependencies {
    implementation(project(":appctr"))
    implementation(fileTree(mapOf("include" to listOf("*.aar", "*.jar"), "dir" to "libs")))
    implementation(libs.core.ktx)
    implementation(libs.appcompat)
    implementation(libs.material)
    implementation(libs.constraintlayout)
    implementation(libs.navigation.fragment.ktx)
    implementation(libs.navigation.ui.ktx)
    testImplementation(libs.junit)
    androidTestImplementation(libs.androidx.test.ext.junit)
    androidTestImplementation(libs.espresso.core)
}
